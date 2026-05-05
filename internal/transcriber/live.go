//go:build !android

package transcriber

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	shared "github.com/asolopovas/wt/internal"
	"github.com/asolopovas/wt/internal/diarizer"
	"github.com/asolopovas/wt/internal/models"
)

func Live(language, modelSize, _ string, threads int) error {
	bin, err := findSherpaASRBinary()
	if err != nil {
		return fmt.Errorf("live: %w", err)
	}

	chunkSamples := 2 * WhisperSampleRate
	bufBytes := chunkSamples * 2

	cmd, err := microphoneCommand()
	if err != nil {
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("creating mic pipe: %w", err)
	}
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("starting ffmpeg for microphone: %w", err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	fmt.Println("Listening (sherpa-onnx Moonshine)... Ctrl+C to stop")

	tmpDir, err := os.MkdirTemp("", "wt-live-*")
	if err != nil {
		return fmt.Errorf("temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pcm := make([]byte, bufBytes)
	samples := make([]float32, chunkSamples)
	var transcript strings.Builder
	chunkIdx := 0

	moonshineDir := os.Getenv("WT_MOONSHINE_DIR")
	if moonshineDir == "" {
		moonshineDir = models.DirForID("moonshine-tiny-en-int8")
	}

	hooks := Hooks{}

	done := make(chan error, 1)
	go func() {
		for {
			select {
			case <-ctx.Done():
				done <- nil
				return
			default:
			}
			if _, err := io.ReadFull(stdout, pcm); err != nil {
				if !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
					fmt.Fprintf(os.Stderr, "\nmic read: %v\n", err)
				}
				done <- err
				return
			}
			samples = samples[:chunkSamples]
			pcmChunkToFloat32(pcm, samples)

			wavPath := filepath.Join(tmpDir, fmt.Sprintf("chunk-%d.wav", chunkIdx))
			if werr := WritePCM16WAV(wavPath, samples, WhisperSampleRate); werr != nil {
				fmt.Fprintf(os.Stderr, "\nwav: %v\n", werr)
				continue
			}
			out, rerr := liveDecode(ctx, bin, moonshineDir, wavPath)
			_ = os.Remove(wavPath)
			if rerr != nil {
				continue
			}
			text := strings.TrimSpace(out)
			if text != "" {
				fmt.Print(text + " ")
				transcript.WriteString(text)
				transcript.WriteString(" ")
			}
			chunkIdx++
		}
	}()

	select {
	case <-sigCh:
	case <-done:
	}
	cancel()
	fmt.Println("\nStopping...")

	if text := strings.TrimSpace(transcript.String()); text != "" {
		if err := os.WriteFile("log.txt", []byte(text), 0o644); err != nil {
			return fmt.Errorf("saving transcript: %w", err)
		}
		fmt.Println("Transcription saved to log.txt")
	}

	_ = language
	_ = modelSize
	_ = threads
	_ = diarizer.SupportsExternalBackend
	_ = hooks
	_ = time.Now
	return nil
}

func liveDecode(ctx context.Context, bin, modelDir, wavPath string) (string, error) {
	args := []string{
		"--moonshine-preprocessor=" + filepath.Join(modelDir, "preprocess.onnx"),
		"--moonshine-encoder=" + filepath.Join(modelDir, "encode.int8.onnx"),
		"--moonshine-uncached-decoder=" + filepath.Join(modelDir, "uncached_decode.int8.onnx"),
		"--moonshine-cached-decoder=" + filepath.Join(modelDir, "cached_decode.int8.onnx"),
		"--tokens=" + filepath.Join(modelDir, "tokens.txt"),
		"--num-threads=4",
		"--provider=cpu",
		wavPath,
	}
	c := exec.CommandContext(ctx, bin, args...)
	shared.HideWindow(c)
	var out bytes.Buffer
	c.Stdout = &out
	c.Stderr = nil
	if err := c.Run(); err != nil {
		return "", err
	}
	return parseLiveSherpaOutput(out.String()), nil
}

func parseLiveSherpaOutput(s string) string {
	for _, line := range strings.Split(s, "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "{") && strings.Contains(t, "\"text\":") {
			i := strings.Index(t, "\"text\":")
			rest := t[i+len("\"text\":"):]
			rest = strings.TrimLeft(rest, " ")
			if !strings.HasPrefix(rest, "\"") {
				continue
			}
			rest = rest[1:]
			j := strings.Index(rest, "\"")
			if j < 0 {
				continue
			}
			return rest[:j]
		}
	}
	return ""
}

func pcmChunkToFloat32(buf []byte, out []float32) {
	const scale = 1.0 / 32768.0
	n := len(buf) / 2
	if len(out) < n {
		n = len(out)
	}
	for i := 0; i < n; i++ {
		s := int16(uint16(buf[i*2]) | uint16(buf[i*2+1])<<8)
		out[i] = float32(s) * scale
	}
}

func microphoneCommand() (*exec.Cmd, error) {
	ff := findFFmpeg()
	if ff == "" {
		return nil, fmt.Errorf("ffmpeg not found; install ffmpeg for live transcription")
	}
	switch runtime.GOOS {
	case "windows":
		return exec.Command(
			ff,
			"-loglevel", "quiet",
			"-f", "dshow",
			"-i", "audio=default",
			"-ar", "16000", "-ac", "1",
			"-sample_fmt", "s16", "-f", "s16le", "-",
		), nil
	case "darwin":
		return exec.Command(
			ff,
			"-loglevel", "quiet",
			"-f", "avfoundation",
			"-i", ":default",
			"-ar", "16000", "-ac", "1",
			"-sample_fmt", "s16", "-f", "s16le", "-",
		), nil
	default:
		return exec.Command(
			ff,
			"-loglevel", "quiet",
			"-f", "pulse", "-i", "default",
			"-ar", "16000", "-ac", "1",
			"-sample_fmt", "s16", "-f", "s16le", "-",
		), nil
	}
}
