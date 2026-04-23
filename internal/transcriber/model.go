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

func ResolveModelPath(modelSize, modelPath string) (string, error) {
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
	pterm.Info.Printf("Downloading model '%s' from %s\n", modelSize, url)
	pterm.FgDarkGray.Println("This may take a few minutes on first run.")

	if err := downloadFile(path, url); err != nil {
		_ = os.Remove(path)
		return "", fmt.Errorf("downloading model: %w", err)
	}

	ui.Done(fmt.Sprintf("Model saved to %s", path))
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
	if err := downloadFile(path, vadModelURL); err != nil {
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

func downloadFile(dst, url string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	f, err := os.Create(dst)
	if err != nil {
		return err
	}

	size := resp.ContentLength
	reader := io.Reader(resp.Body)

	if size > 0 {
		pb, err := pterm.DefaultProgressbar.
			WithTitle("Downloading").
			WithTotal(int(size / (1024 * 1024))).
			WithShowCount(true).
			Start()

		if err == nil && pb != nil {
			reader = &progressReader{
				reader:    resp.Body,
				totalSize: size,
				bar:       pb,
			}
		}
	}

	if _, err := io.Copy(f, reader); err != nil {
		_ = f.Close()
		return err
	}

	return f.Close()
}

type progressReader struct {
	reader    io.Reader
	totalSize int64
	written   int64
	lastMB    int
	bar       *pterm.ProgressbarPrinter
}

func (pr *progressReader) Read(p []byte) (int, error) {
	n, err := pr.reader.Read(p)
	pr.written += int64(n)
	mb := int(pr.written / (1024 * 1024))
	if mb > pr.lastMB {
		pr.bar.Add(mb - pr.lastMB)
		pr.lastMB = mb
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
