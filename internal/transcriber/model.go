package transcriber

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/pterm/pterm"

	shared "github.com/asolopovas/wt/internal"
	"github.com/asolopovas/wt/internal/ui"
	whisper "github.com/ggerganov/whisper.cpp/bindings/go/pkg/whisper"
)

type Model struct {
	whisper.Model
}

const modelURLBase = "https://huggingface.co/ggerganov/whisper.cpp/resolve/main"

var ModelFiles = map[string]string{
	"tiny":             "ggml-tiny.bin",
	"tiny.en":          "ggml-tiny.en.bin",
	"base":             "ggml-base.bin",
	"base.en":          "ggml-base.en.bin",
	"small":            "ggml-small.bin",
	"small.en":         "ggml-small.en.bin",
	"medium":           "ggml-medium.bin",
	"medium.en":        "ggml-medium.en.bin",
	"large-v1":         "ggml-large-v1.bin",
	"large-v2":         "ggml-large-v2.bin",
	"large-v3":         "ggml-large-v3.bin",
	"large":            "ggml-large-v3.bin",
	"distil-small.en":  "ggml-distil-small.en.bin",
	"distil-medium.en": "ggml-distil-medium.en.bin",
	"distil-large-v2":  "ggml-distil-large-v2.bin",
	"distil-large-v3":  "ggml-distil-large-v3.bin",
	"large-v3-turbo":   "ggml-large-v3-turbo.bin",
	"turbo":            "ggml-large-v3-turbo.bin",
}

func ValidModelNames() []string {
	names := make([]string, 0, len(ModelFiles))
	for k := range ModelFiles {
		names = append(names, k)
	}
	slices.Sort(names)
	return names
}

type DownloadProgress func(downloaded, total int64)

func ResolveModelPath(modelSize, modelPath string) (string, error) {
	return ResolveModelPathWithProgress(modelSize, modelPath, nil)
}

func ResolveModelPathWithProgress(modelSize, modelPath string, prog DownloadProgress) (string, error) {
	if modelPath != "" {
		if _, err := os.Stat(modelPath); err != nil {
			return "", fmt.Errorf("model file not found: %s", modelPath)
		}
		return modelPath, nil
	}

	filename, ok := ModelFiles[modelSize]
	if !ok {
		return "", fmt.Errorf("unknown model size %q", modelSize)
	}

	dir := shared.ModelsDir()
	path := filepath.Join(dir, filename)

	if _, err := os.Stat(path); err == nil {
		return path, nil
	}

	for _, legacyDir := range legacyModelDirs() {
		oldPath := filepath.Join(legacyDir, filename)
		if _, err := os.Stat(oldPath); err == nil {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return "", fmt.Errorf("creating models dir: %w", err)
			}
			pterm.Info.Printf("Migrating model from %s...\n", oldPath)
			if err := os.Rename(oldPath, path); err != nil {
				ui.Warn(fmt.Sprintf("could not migrate model: %v", err))
			} else {
				return path, nil
			}
		}
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("creating models dir: %w", err)
	}

	url := modelURLBase + "/" + filename
	if prog == nil {
		pterm.Info.Printf("Downloading model '%s' from %s\n", modelSize, url)
		pterm.FgDarkGray.Println("This may take a few minutes on first run.")
	}

	if err := downloadFile(path, url, prog); err != nil {
		return "", fmt.Errorf("downloading model: %w", err)
	}

	if prog == nil {
		ui.Done(fmt.Sprintf("Model saved to %s", path))
	}
	return path, nil
}

const (
	vadModelFile = "ggml-silero-v6.2.0.bin"
	vadModelURL  = "https://huggingface.co/ggml-org/whisper-vad/resolve/main/" + vadModelFile
)

func ResolveVADModelPath() (string, error) {
	dir := shared.ModelsDir()
	path := filepath.Join(dir, vadModelFile)

	if _, err := os.Stat(path); err == nil {
		return path, nil
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("creating models dir: %w", err)
	}

	ui.Stage("Downloading VAD model...")
	if err := downloadFile(path, vadModelURL, nil); err != nil {
		_ = os.Remove(path)
		return "", fmt.Errorf("downloading VAD model: %w", err)
	}

	ui.Done(fmt.Sprintf("VAD model saved to %s", path))
	return path, nil
}

func legacyModelDirs() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	return []string{
		filepath.Join(home, ".wt", "models"),
		filepath.Join(home, ".cache", "wt"),
	}
}

func downloadFile(dst, url string, prog DownloadProgress) error {
	const maxReadAttempts = 8
	const maxDialAttempts = 30
	tmp := dst + ".part"

	client := &http.Client{Timeout: 0}

	var pb *pterm.ProgressbarPrinter
	var totalSize int64
	var lastErr error
	readAttempt := 0
	dialAttempt := 0

	for {
		if readAttempt >= maxReadAttempts || dialAttempt >= maxDialAttempts {
			break
		}

		var offset int64
		if st, err := os.Stat(tmp); err == nil {
			offset = st.Size()
		}

		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return err
		}
		if offset > 0 {
			req.Header.Set("Range", fmt.Sprintf("bytes=%d-", offset))
		}

		resp, err := client.Do(req)
		if err != nil {
			dialAttempt++
			lastErr = err
			if prog != nil {
				prog(-int64(dialAttempt), 0)
			} else {
				ui.Warn(fmt.Sprintf("connect failed (%d/%d): %v — retrying", dialAttempt, maxDialAttempts, err))
			}
			sleep := time.Duration(dialAttempt) * 2 * time.Second
			if sleep > 15*time.Second {
				sleep = 15 * time.Second
			}
			time.Sleep(sleep)
			continue
		}

		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
			_ = resp.Body.Close()
			return fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
		}

		if resp.StatusCode == http.StatusOK {
			offset = 0
			_ = os.Remove(tmp)
		}

		flag := os.O_WRONLY | os.O_CREATE
		if offset > 0 {
			flag |= os.O_APPEND
		} else {
			flag |= os.O_TRUNC
		}
		f, err := os.OpenFile(tmp, flag, 0o644)
		if err != nil {
			_ = resp.Body.Close()
			return err
		}

		totalSize = resp.ContentLength + offset
		if prog == nil && pb == nil && totalSize > 0 {
			p, perr := pterm.DefaultProgressbar.
				WithTitle("Downloading").
				WithTotal(int(totalSize / (1024 * 1024))).
				WithShowCount(true).
				Start()
			if perr == nil {
				pb = p
				if offset > 0 {
					pb.Add(int(offset / (1024 * 1024)))
				}
			}
		}

		if prog != nil {
			prog(offset, totalSize)
		}

		reader := io.Reader(resp.Body)
		if pb != nil || prog != nil {
			reader = &progressReader{
				reader:    resp.Body,
				totalSize: totalSize,
				written:   offset,
				lastMB:    int(offset / (1024 * 1024)),
				bar:       pb,
				cb:        prog,
			}
		}

		_, copyErr := io.Copy(f, reader)
		_ = f.Close()
		_ = resp.Body.Close()

		if copyErr == nil {
			if prog != nil {
				prog(totalSize, totalSize)
			}
			return os.Rename(tmp, dst)
		}

		readAttempt++
		lastErr = copyErr
		if prog != nil {
			prog(-int64(readAttempt), totalSize)
		} else {
			ui.Warn(fmt.Sprintf("download interrupted (%d/%d): %v — retrying", readAttempt, maxReadAttempts, copyErr))
		}
		sleep := time.Duration(readAttempt) * 2 * time.Second
		if sleep > 15*time.Second {
			sleep = 15 * time.Second
		}
		time.Sleep(sleep)
	}

	partSize := int64(0)
	if st, err := os.Stat(tmp); err == nil {
		partSize = st.Size()
	}
	return fmt.Errorf("gave up after %d read / %d connect retries (partial download preserved: %.1f MB at %s.part — re-run to resume): %w",
		readAttempt, dialAttempt, float64(partSize)/(1024*1024), filepath.Base(dst), lastErr)
}

type progressReader struct {
	reader    io.Reader
	totalSize int64
	written   int64
	lastMB    int
	bar       *pterm.ProgressbarPrinter
	cb        DownloadProgress
	lastCb    time.Time
}

func (pr *progressReader) Read(p []byte) (int, error) {
	n, err := pr.reader.Read(p)
	pr.written += int64(n)
	mb := int(pr.written / (1024 * 1024))
	if mb > pr.lastMB {
		if pr.bar != nil {
			pr.bar.Add(mb - pr.lastMB)
		}
		pr.lastMB = mb
	}
	if pr.cb != nil {
		now := time.Now()
		if now.Sub(pr.lastCb) >= 250*time.Millisecond {
			pr.cb(pr.written, pr.totalSize)
			pr.lastCb = now
		}
	}
	return n, err
}

func LoadModel(modelSize, modelPath string, threads int) (*Model, error) {
	path, err := ResolveModelPath(modelSize, modelPath)
	if err != nil {
		return nil, err
	}

	exePath, err := os.Executable()
	if err == nil {
		whisper.BackendSetSearchPath(filepath.Dir(exePath))
	}

	whisper.SetLogQuiet(true)

	spinner := ui.Spinner(fmt.Sprintf("Loading model '%s'...", modelSize))
	start := time.Now()

	m, err := whisper.New(path)
	if err != nil {
		_ = spinner.Stop()
		ui.Crossf("Loading model '%s' FAILED", modelSize)
		return nil, fmt.Errorf("loading model: %w", err)
	}

	_ = spinner.Stop()

	gpuFound := false
	devices := whisper.BackendDevices()
	for _, dev := range devices {
		if dev.Type == "GPU" || dev.Type == "iGPU" {
			gpuFound = true
			ui.Tickf("Model loaded (%s, %s, %.1fs)", modelSize, dev.Type, time.Since(start).Seconds())
			ui.Debug("Device", dev.Description)
			usedMB := dev.TotalMB - dev.FreeMB
			ui.Debug("VRAM", fmt.Sprintf("%d/%d MB", usedMB, dev.TotalMB))
		}
	}
	if !gpuFound {
		ui.Tickf("Model loaded (%s, CPU, %.1fs)", modelSize, time.Since(start).Seconds())
	}
	ui.Debug("Threads", fmt.Sprintf("%d", threads))

	return &Model{Model: m}, nil
}
