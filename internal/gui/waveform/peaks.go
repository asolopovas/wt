//go:build !android

package waveform

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sync"

	shared "github.com/asolopovas/wt/internal"
	"github.com/asolopovas/wt/internal/transcriber"
)

const (
	peakBuckets    = 2000
	peaksSampleRate = 16000
	peaksFileMagic = uint32(0x57544b50) // "WTKP"
	peaksFileVer   = uint32(1)
)

type Peaks struct {
	Min      []float32
	Max      []float32
	Duration float64
}

func (p *Peaks) Len() int {
	if p == nil {
		return 0
	}
	return len(p.Max)
}

func cacheKey(path string) (string, int64, error) {
	st, err := os.Stat(path)
	if err != nil {
		return "", 0, err
	}
	h := sha256.New()
	_, _ = fmt.Fprintf(h, "%s\x00%d\x00%d", path, st.Size(), st.ModTime().UnixNano())
	return hex.EncodeToString(h.Sum(nil))[:32], st.Size(), nil
}

func cachePath(key string) string {
	return filepath.Join(shared.CacheDir(), "peaks", key+".bin")
}

func loadCached(key string) (*Peaks, bool) {
	f, err := os.Open(cachePath(key))
	if err != nil {
		return nil, false
	}
	defer func() { _ = f.Close() }()

	var hdr struct {
		Magic, Ver uint32
		N          uint32
		Duration   float64
	}
	if err := binary.Read(f, binary.LittleEndian, &hdr); err != nil {
		return nil, false
	}
	if hdr.Magic != peaksFileMagic || hdr.Ver != peaksFileVer || hdr.N == 0 || hdr.N > 1<<20 {
		return nil, false
	}
	min := make([]float32, hdr.N)
	max := make([]float32, hdr.N)
	if err := binary.Read(f, binary.LittleEndian, &min); err != nil {
		return nil, false
	}
	if err := binary.Read(f, binary.LittleEndian, &max); err != nil {
		return nil, false
	}
	return &Peaks{Min: min, Max: max, Duration: hdr.Duration}, true
}

func saveCached(key string, p *Peaks) {
	dir := filepath.Dir(cachePath(key))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	tmp := cachePath(key) + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return
	}
	hdr := struct {
		Magic, Ver uint32
		N          uint32
		Duration   float64
	}{peaksFileMagic, peaksFileVer, uint32(len(p.Max)), p.Duration}
	if err := binary.Write(f, binary.LittleEndian, &hdr); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return
	}
	if err := binary.Write(f, binary.LittleEndian, p.Min); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return
	}
	if err := binary.Write(f, binary.LittleEndian, p.Max); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return
	}
	_ = os.Rename(tmp, cachePath(key))
}

var (
	mu       sync.Mutex
	inflight = map[string]chan struct{}{}
)

// Extract loads or computes peaks for path. Safe to call from any goroutine.
// Concurrent calls for the same path coalesce.
func Extract(path string) (*Peaks, error) {
	key, _, err := cacheKey(path)
	if err != nil {
		return nil, err
	}
	if p, ok := loadCached(key); ok {
		return p, nil
	}

	mu.Lock()
	if ch, ok := inflight[key]; ok {
		mu.Unlock()
		<-ch
		if p, ok := loadCached(key); ok {
			return p, nil
		}
		return nil, errors.New("peaks: in-flight extract failed")
	}
	ch := make(chan struct{})
	inflight[key] = ch
	mu.Unlock()
	defer func() {
		mu.Lock()
		delete(inflight, key)
		close(ch)
		mu.Unlock()
	}()

	p, err := decodePeaks(path)
	if err != nil {
		return nil, err
	}
	saveCached(key, p)
	return p, nil
}

func decodePeaks(path string) (*Peaks, error) {
	bin := transcriber.FindFFmpeg()
	if bin == "" {
		return nil, errors.New("ffmpeg not found")
	}
	cmd := exec.Command(bin,
		"-hide_banner", "-loglevel", "error", "-nostdin",
		"-i", path,
		"-f", "s16le", "-ac", "1", "-ar", fmt.Sprintf("%d", peaksSampleRate),
		"-",
	)
	shared.HideWindow(cmd)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	const chunkSamples = 4096
	buf := make([]byte, chunkSamples*2)

	// First pass: stream all samples into a slice (capped). For typical files (<2h)
	// memory is ~230MB max @16kHz mono int16. We then bucket into peaks.
	var all []int16
	for {
		n, rerr := io.ReadFull(stdout, buf)
		for i := 0; i+1 < n; i += 2 {
			v := int16(binary.LittleEndian.Uint16(buf[i:]))
			all = append(all, v)
		}
		if rerr == io.EOF || rerr == io.ErrUnexpectedEOF {
			break
		}
		if rerr != nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			return nil, rerr
		}
	}
	if err := cmd.Wait(); err != nil {
		return nil, err
	}
	if len(all) == 0 {
		return nil, errors.New("peaks: empty audio")
	}

	dur := float64(len(all)) / float64(peaksSampleRate)
	n := peakBuckets
	if len(all) < n {
		n = len(all)
	}
	mn := make([]float32, n)
	mx := make([]float32, n)
	step := float64(len(all)) / float64(n)
	for i := 0; i < n; i++ {
		s := int(math.Floor(float64(i) * step))
		e := int(math.Floor(float64(i+1) * step))
		if e > len(all) {
			e = len(all)
		}
		if s >= e {
			continue
		}
		var lo, hi int16 = math.MaxInt16, math.MinInt16
		for _, v := range all[s:e] {
			if v < lo {
				lo = v
			}
			if v > hi {
				hi = v
			}
		}
		mn[i] = float32(lo) / 32768
		mx[i] = float32(hi) / 32768
	}
	return &Peaks{Min: mn, Max: mx, Duration: dur}, nil
}
