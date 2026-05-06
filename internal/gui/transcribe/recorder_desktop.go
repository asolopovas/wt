//go:build !android

package transcribe

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sync"
	"time"

	"github.com/asolopovas/wt/internal/transcriber"
)

var (
	recMu        sync.Mutex
	recCmd       *exec.Cmd
	recStdin     io.WriteCloser
	recCancel    context.CancelFunc
	recPath      string
	dshowAudioRE = regexp.MustCompile(`(?m)^\[.*?\]\s*"([^"]+)"\s*\(audio\)`)
)

func desktopRecordingsDir() string {
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, "Documents", "wt", "recordings")
	}
	return os.TempDir()
}

func detectMicDevice(ff string) (string, error) {
	if v := os.Getenv("WT_MIC_DEVICE"); v != "" {
		return v, nil
	}
	switch runtime.GOOS {
	case "windows":
		cmd := exec.Command(ff, "-hide_banner", "-list_devices", "true", "-f", "dshow", "-i", "dummy")
		out, _ := cmd.CombinedOutput()
		m := dshowAudioRE.FindStringSubmatch(string(out))
		if len(m) < 2 {
			return "", errors.New("no DirectShow audio device found; set WT_MIC_DEVICE")
		}
		return m[1], nil
	case "darwin":
		return ":0", nil
	case "linux":
		return "default", nil
	}
	return "", fmt.Errorf("unsupported OS: %s", runtime.GOOS)
}

func micInputArgs(device string) []string {
	switch runtime.GOOS {
	case "windows":
		return []string{"-f", "dshow", "-i", "audio=" + device}
	case "darwin":
		return []string{"-f", "avfoundation", "-i", device}
	case "linux":
		return []string{"-f", "pulse", "-i", device}
	}
	return nil
}

func startRecording() (string, error) {
	recMu.Lock()
	defer recMu.Unlock()
	if recCmd != nil {
		return "", errors.New("recording already in progress")
	}

	ff := transcriber.FindFFmpeg()
	if ff == "" {
		return "", errors.New("ffmpeg not found in PATH")
	}
	device, err := detectMicDevice(ff)
	if err != nil {
		return "", err
	}

	dir := desktopRecordingsDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("recordings dir: %w", err)
	}
	out := filepath.Join(dir, fmt.Sprintf("recording_%s.wav", time.Now().Format("20060102-150405")))

	args := append([]string{"-hide_banner", "-loglevel", "warning"}, micInputArgs(device)...)
	args = append(args, "-ac", "1", "-ar", "16000", "-c:a", "pcm_s16le", "-y", out)

	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, ff, args...)
	stdin, perr := cmd.StdinPipe()
	if perr != nil {
		cancel()
		return "", fmt.Errorf("ffmpeg stdin: %w", perr)
	}
	if err := cmd.Start(); err != nil {
		cancel()
		return "", fmt.Errorf("starting ffmpeg: %w", err)
	}

	recCmd, recStdin, recCancel = cmd, stdin, cancel
	recPath = out
	_ = device
	return out, nil
}

func stopRecording() (string, error) {
	recMu.Lock()
	cmd := recCmd
	stdin := recStdin
	cancel := recCancel
	out := recPath
	recCmd, recStdin, recCancel = nil, nil, nil
	recPath = ""
	recMu.Unlock()
	if cmd == nil {
		return "", errors.New("not recording")
	}
	if stdin != nil {
		_, _ = stdin.Write([]byte("q\n"))
		_ = stdin.Close()
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		cancel()
		<-done
	}
	if _, err := os.Stat(out); err != nil {
		return "", fmt.Errorf("recording file missing: %w", err)
	}
	return out, nil
}

func isRecording() bool {
	recMu.Lock()
	defer recMu.Unlock()
	return recCmd != nil
}
