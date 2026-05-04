//go:build !android

package transcriber

import (
	"bufio"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	shared "github.com/asolopovas/wt/internal"
)

type StreamOptions struct {
	ChunkSamples int
	CacheWAVPath string
}

type AudioStream struct {
	cmd          *exec.Cmd
	stdout       io.ReadCloser
	stderr       *strings.Builder
	stderrPipe   io.ReadCloser
	durationSec  float64
	chunkSamples int
	chunkBytes   int
	readBuf      []byte
	floatBuf     []float32
	emittedSamps int
	teeWriter    *teeWAVWriter
	teeOnce      sync.Once
	closed       bool
}

func samplesPerChunk(chunkSec float64) int {
	if chunkSec <= 0 {
		chunkSec = defaultChunkSec
	}
	return int(chunkSec * float64(WhisperSampleRate))
}

func OpenAudioStream(ctx context.Context, absPath string, opts StreamOptions) (*AudioStream, error) {
	ff := findFFmpeg()
	if ff == "" {
		return nil, fmt.Errorf("ffmpeg not found")
	}
	chunkN := opts.ChunkSamples
	if chunkN <= 0 {
		chunkN = samplesPerChunk(chunkSec())
	}
	durMs := ProbeDurationMs(absPath)
	durSec := float64(durMs) / 1000.0

	cmd := exec.CommandContext(ctx, ff,
		"-loglevel", "error",
		"-nostdin",
		"-i", absPath,
		"-vn",
		"-ar", "16000",
		"-ac", "1",
		"-f", "s16le",
		"pipe:1",
	)
	shared.HideWindow(cmd)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("ffmpeg stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		_ = stdout.Close()
		return nil, fmt.Errorf("ffmpeg stderr pipe: %w", err)
	}

	stream := &AudioStream{
		cmd:          cmd,
		stdout:       stdout,
		stderrPipe:   stderr,
		stderr:       &strings.Builder{},
		durationSec:  durSec,
		chunkSamples: chunkN,
		chunkBytes:   chunkN * 2,
		readBuf:      make([]byte, chunkN*2),
		floatBuf:     make([]float32, chunkN),
	}

	if opts.CacheWAVPath != "" {
		tw, terr := newTeeWAVWriter(opts.CacheWAVPath, WhisperSampleRate)
		if terr != nil {
			stream.log("tee init failed: " + terr.Error())
		} else {
			stream.teeWriter = tw
		}
	}

	if err := cmd.Start(); err != nil {
		_ = stdout.Close()
		_ = stderr.Close()
		if stream.teeWriter != nil {
			_ = stream.teeWriter.Abort()
		}
		return nil, fmt.Errorf("ffmpeg start: %w", err)
	}

	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := stream.stderrPipe.Read(buf)
			if n > 0 {
				stream.stderr.Write(buf[:n])
			}
			if err != nil {
				return
			}
		}
	}()

	return stream, nil
}

func (s *AudioStream) Duration() float64 { return s.durationSec }

func (s *AudioStream) log(msg string) {
	_ = msg
}

func (s *AudioStream) Next() ([]float32, error) {
	if s.closed {
		return nil, io.EOF
	}
	n, err := io.ReadFull(s.stdout, s.readBuf)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("ffmpeg read: %w", err)
	}
	n -= n % 2
	if n == 0 {
		return nil, io.EOF
	}
	if s.teeWriter != nil {
		if werr := s.teeWriter.Write(s.readBuf[:n]); werr != nil {
			s.log("tee write failed: " + werr.Error())
			_ = s.teeWriter.Abort()
			s.teeWriter = nil
		}
	}
	const scale = 1.0 / float32(math.MaxInt16)
	count := n / 2
	out := s.floatBuf[:count]
	for i := 0; i < count; i++ {
		v := int16(binary.LittleEndian.Uint16(s.readBuf[i*2:]))
		out[i] = float32(v) * scale
	}
	s.emittedSamps += count
	return out, nil
}

func (s *AudioStream) EmittedSeconds() float64 {
	return float64(s.emittedSamps) / float64(WhisperSampleRate)
}

func (s *AudioStream) Close() error {
	if s.closed {
		return nil
	}
	s.closed = true
	_ = s.stdout.Close()
	waitErr := s.cmd.Wait()
	var teeErr error
	s.teeOnce.Do(func() {
		if s.teeWriter != nil {
			teeErr = s.teeWriter.Finalize()
		}
	})
	if waitErr != nil {
		stderr := strings.TrimSpace(s.stderr.String())
		if stderr != "" {
			return fmt.Errorf("ffmpeg: %w: %s", waitErr, stderr)
		}
		return fmt.Errorf("ffmpeg: %w", waitErr)
	}
	if teeErr != nil {
		return fmt.Errorf("wav tee: %w", teeErr)
	}
	return nil
}

func (s *AudioStream) Abort() {
	if s.closed {
		return
	}
	s.closed = true
	_ = s.stdout.Close()
	_ = s.cmd.Process.Kill()
	_, _ = s.cmd.Process.Wait()
	s.teeOnce.Do(func() {
		if s.teeWriter != nil {
			_ = s.teeWriter.Abort()
		}
	})
}

type teeWAVWriter struct {
	path       string
	tmpPath    string
	file       *os.File
	bw         *bufio.Writer
	sampleRate int
	written    uint32
	finalized  bool
}

func newTeeWAVWriter(path string, sampleRate int) (*teeWAVWriter, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return nil, err
	}
	bw := bufio.NewWriterSize(f, 256*1024)
	hdr := wavHeader(0, sampleRate)
	if _, err := bw.Write(hdr[:]); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return nil, err
	}
	return &teeWAVWriter{path: path, tmpPath: tmp, file: f, bw: bw, sampleRate: sampleRate}, nil
}

func (t *teeWAVWriter) Write(pcm []byte) error {
	n, err := t.bw.Write(pcm)
	t.written += uint32(n)
	return err
}

func (t *teeWAVWriter) Finalize() error {
	if t.finalized {
		return nil
	}
	t.finalized = true
	if err := t.bw.Flush(); err != nil {
		_ = t.file.Close()
		_ = os.Remove(t.tmpPath)
		return err
	}
	hdr := wavHeader(t.written, t.sampleRate)
	if _, err := t.file.WriteAt(hdr[:], 0); err != nil {
		_ = t.file.Close()
		_ = os.Remove(t.tmpPath)
		return err
	}
	if err := t.file.Close(); err != nil {
		_ = os.Remove(t.tmpPath)
		return err
	}
	return os.Rename(t.tmpPath, t.path)
}

func (t *teeWAVWriter) Abort() error {
	if t.finalized {
		return nil
	}
	t.finalized = true
	_ = t.bw.Flush()
	_ = t.file.Close()
	return os.Remove(t.tmpPath)
}

func wavHeader(dataSize uint32, sampleRate int) [44]byte {
	const numChannels = 1
	const bitsPerSample = 16
	byteRate := sampleRate * numChannels * bitsPerSample / 8
	blockAlign := numChannels * bitsPerSample / 8
	chunkSize := uint32(36) + dataSize

	var hdr [44]byte
	copy(hdr[0:4], "RIFF")
	binary.LittleEndian.PutUint32(hdr[4:8], chunkSize)
	copy(hdr[8:16], "WAVEfmt ")
	binary.LittleEndian.PutUint32(hdr[16:20], 16)
	binary.LittleEndian.PutUint16(hdr[20:22], 1)
	binary.LittleEndian.PutUint16(hdr[22:24], uint16(numChannels))
	binary.LittleEndian.PutUint32(hdr[24:28], uint32(sampleRate))
	binary.LittleEndian.PutUint32(hdr[28:32], uint32(byteRate))
	binary.LittleEndian.PutUint16(hdr[32:34], uint16(blockAlign))
	binary.LittleEndian.PutUint16(hdr[34:36], uint16(bitsPerSample))
	copy(hdr[36:40], "data")
	binary.LittleEndian.PutUint32(hdr[40:44], dataSize)
	return hdr
}

func StreamingEnabled() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("WT_STREAM")))
	switch v {
	case "0", "false", "off", "no":
		return false
	default:
		return true
	}
}
