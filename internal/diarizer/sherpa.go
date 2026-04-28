package diarizer

import (
	"archive/zip"
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"

	shared "github.com/asolopovas/wt/internal"
)

type sherpaDiarizer struct {
	binPath  string
	segModel string
	embModel string
}

func sherpaBinName() string {
	if runtime.GOOS == "windows" {
		return "sherpa-onnx-offline-speaker-diarization.exe"
	}
	if runtime.GOOS == "android" {
		return "libsherpa-diar.so"
	}
	return "sherpa-onnx-offline-speaker-diarization"
}

func newSherpaDiarizer() (Backend, error) {
	bin, err := findSherpaBinary()
	if err != nil {
		return nil, err
	}

	seg, emb, err := resolveSherpaModels()
	if err != nil {
		return nil, err
	}

	return &sherpaDiarizer{binPath: bin, segModel: seg, embModel: emb}, nil
}

const (
	sherpaSegURL = "https://huggingface.co/csukuangfj/sherpa-onnx-pyannote-segmentation-3-0/resolve/main/model.onnx"
	sherpaEmbURL = "https://github.com/k2-fsa/sherpa-onnx/releases/download/speaker-recongition-models/3dspeaker_speech_eres2net_base_sv_zh-cn_3dspeaker_16k.onnx"
)

// SherpaModelPaths returns the canonical (seg, emb) paths inside ModelsDir.
func SherpaModelPaths() (string, string) {
	root := shared.ModelsDir()
	if runtime.GOOS == "android" {
		return filepath.Join(root, "seg.onnx"), filepath.Join(root, "emb.onnx")
	}
	return filepath.Join(root, "sherpa-onnx-pyannote-segmentation-3-0", "model.onnx"),
		filepath.Join(root, "3dspeaker.onnx")
}

// EnsureSherpaModels makes sure both pyannote-segmentation and 3dspeaker
// embedding models are present on disk. Tries APK assets on Android first
// (no network), then downloads from huggingface with progress reporting.
// progress is called as progress(name, downloaded, total); name is "seg" or "emb".
func EnsureSherpaModels(progress func(name string, downloaded, total int64)) error {
	seg, emb := SherpaModelPaths()

	if runtime.GOOS == "android" && (!fileExists(seg) || !fileExists(emb)) {
		if err := installSherpaModelsFromAssets(shared.ModelsDir()); err == nil {
			seg, emb = SherpaModelPaths()
		}
	}

	if !fileExists(seg) {
		cb := func(d, t int64) {
			if progress != nil {
				progress("seg", d, t)
			}
		}
		if err := shared.DownloadFile(seg, sherpaSegURL, cb); err != nil {
			return fmt.Errorf("downloading segmentation model: %w", err)
		}
	}
	if !fileExists(emb) {
		cb := func(d, t int64) {
			if progress != nil {
				progress("emb", d, t)
			}
		}
		if err := shared.DownloadFile(emb, sherpaEmbURL, cb); err != nil {
			return fmt.Errorf("downloading embedding model: %w", err)
		}
	}
	return nil
}

func resolveSherpaModels() (string, string, error) {
	seg, emb := SherpaModelPaths()
	segOK := fileExists(seg)
	embOK := fileExists(emb)

	if runtime.GOOS == "android" && (!segOK || !embOK) {
		if err := installSherpaModelsFromAssets(shared.ModelsDir()); err == nil {
			seg, emb = SherpaModelPaths()
			segOK = fileExists(seg)
			embOK = fileExists(emb)
		}
	}

	if !segOK {
		return "", "", fmt.Errorf("segmentation model missing at %s (call EnsureSherpaModels first)", seg)
	}
	if !embOK {
		return "", "", fmt.Errorf("embedding model missing at %s (call EnsureSherpaModels first)", emb)
	}
	return seg, emb, nil
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

func findSherpaBinary() (string, error) {
	name := sherpaBinName()

	exePath, err := os.Executable()
	if err == nil {
		c := filepath.Join(filepath.Dir(exePath), name)
		if fileExists(c) {
			return c, nil
		}
	}

	if runtime.GOOS == "android" {
		for _, dir := range androidNativeLibDirs() {
			c := filepath.Join(dir, name)
			if fileExists(c) {
				return c, nil
			}
		}
	}

	c := filepath.Join(shared.Dir(), name)
	if fileExists(c) {
		return c, nil
	}

	cwd, err := os.Getwd()
	if err == nil {
		for _, p := range []string{
			filepath.Join(cwd, "dist", "sherpa", "win", "bin", name),
			filepath.Join(cwd, "dist", "bin", name),
		} {
			if fileExists(p) {
				return p, nil
			}
		}
	}

	if p, err := exec.LookPath(name); err == nil {
		return p, nil
	}

	return "", fmt.Errorf("%s not found", name)
}

func androidNativeLibDirs() []string {
	var dirs []string
	if v := os.Getenv("ANDROID_NATIVE_LIBS_DIR"); v != "" {
		dirs = append(dirs, v)
	}
	for _, env := range []string{"LD_LIBRARY_PATH", "LIB_DIR"} {
		v := os.Getenv(env)
		for _, p := range strings.Split(v, ":") {
			if p != "" {
				dirs = append(dirs, p)
			}
		}
	}
	if matches, err := filepath.Glob("/data/app/*/com.asolopovas.wtranscribe-*/lib/arm64"); err == nil {
		dirs = append(dirs, matches...)
	}
	if matches, err := filepath.Glob("/data/app/com.asolopovas.wtranscribe-*/lib/arm64"); err == nil {
		dirs = append(dirs, matches...)
	}
	return dirs
}

func installSherpaModelsFromAssets(dest string) error {
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}
	apkPath, err := findApkPath()
	if err != nil {
		return err
	}
	zr, err := zip.OpenReader(apkPath)
	if err != nil {
		return err
	}
	defer zr.Close()

	want := map[string]string{
		"assets/sherpa-models/seg.onnx": filepath.Join(dest, "seg.onnx"),
		"assets/sherpa-models/emb.onnx": filepath.Join(dest, "emb.onnx"),
	}
	found := 0
	for _, f := range zr.File {
		out, ok := want[f.Name]
		if !ok {
			continue
		}
		if err := extractZipEntry(f, out); err != nil {
			return fmt.Errorf("extract %s: %w", f.Name, err)
		}
		found++
	}
	if found < len(want) {
		return fmt.Errorf("APK missing sherpa model assets (found %d/%d)", found, len(want))
	}
	return nil
}

func findApkPath() (string, error) {
	matches, _ := filepath.Glob("/data/app/*/com.asolopovas.wtranscribe-*/base.apk")
	if len(matches) > 0 {
		return matches[0], nil
	}
	matches, _ = filepath.Glob("/data/app/com.asolopovas.wtranscribe-*/base.apk")
	if len(matches) > 0 {
		return matches[0], nil
	}
	return "", fmt.Errorf("apk not found")
}

func extractZipEntry(f *zip.File, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	tmp := dst + ".tmp"
	w, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err := io.Copy(w, rc); err != nil {
		w.Close()
		os.Remove(tmp)
		return err
	}
	if err := w.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, dst)
}

func (d *sherpaDiarizer) Name() string { return "sherpa-onnx-pyannote" }

var sherpaSegRE = regexp.MustCompile(`^\s*([0-9]+\.[0-9]+)\s+--\s+([0-9]+\.[0-9]+)\s+speaker_(\d+)\s*$`)

var sherpaProgRE = regexp.MustCompile(`progress\s+([0-9]+\.[0-9]+)%`)

func (d *sherpaDiarizer) Diarize(ctx context.Context, wavPath string, numSpeakers int, audioDurSec float64, progress ProgressFunc) ([]Segment, error) {
	args := []string{
		"--segmentation.pyannote-model=" + d.segModel,
		"--embedding.model=" + d.embModel,
	}
	if numSpeakers > 0 {
		args = append(args, fmt.Sprintf("--clustering.num-clusters=%d", numSpeakers))
	} else {
		args = append(args, "--clustering.cluster-threshold=0.90")
	}
	args = append(args, wavPath)

	cmd := exec.CommandContext(ctx, d.binPath, args...)
	cmd.Env = os.Environ()
	shared.HideWindow(cmd)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("creating stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("creating stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting sherpa-onnx: %w", err)
	}

	var (
		mu          sync.Mutex
		stderrLines []string
		segments    []Segment
		started     bool
	)

	stderrDone := make(chan struct{})
	go func() {
		defer close(stderrDone)
		scanner := bufio.NewScanner(stderr)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			line := scanner.Text()
			if m := sherpaProgRE.FindStringSubmatch(line); m != nil {
				if progress != nil {
					if v, err := strconv.ParseFloat(m[1], 64); err == nil {
						progress(v * 0.99)
					}
				}
				continue
			}
			mu.Lock()
			stderrLines = append(stderrLines, line)
			mu.Unlock()
		}
	}()

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r")
		if !started {
			if strings.HasPrefix(strings.TrimSpace(line), "Started") {
				started = true
			}
			continue
		}
		m := sherpaSegRE.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		start, _ := strconv.ParseFloat(m[1], 64)
		end, _ := strconv.ParseFloat(m[2], 64)
		spk, _ := strconv.Atoi(m[3])
		segments = append(segments, Segment{
			Speaker:  spk,
			StartSec: start,
			EndSec:   end,
		})
	}

	<-stderrDone
	if err := cmd.Wait(); err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("diarization cancelled")
		}
		mu.Lock()
		captured := stderrLines
		mu.Unlock()
		tail := captured
		if len(tail) > 20 {
			tail = tail[len(tail)-20:]
		}
		return nil, fmt.Errorf("sherpa-onnx failed:\n%s", strings.Join(tail, "\n"))
	}

	if progress != nil {
		progress(100)
	}
	return segments, nil
}
