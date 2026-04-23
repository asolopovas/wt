package transcriber

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
)

func Live(language, modelSize, modelPath string, threads int) error {
	model, err := LoadModel(modelSize, modelPath, threads)
	if err != nil {
		return err
	}
	defer func() {
		_ = model.Close()
	}()

	cmd, err := microphoneCommand()
	if err != nil {
		return err
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("creating pipe: %w", err)
	}
	cmd.Stderr = nil

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("starting ffmpeg for microphone capture: %w; make sure ffmpeg is installed and a microphone is available", err)
	}

	fmt.Println("Listening... (Ctrl+C to stop)")

	var accumulated strings.Builder
	chunkSamples := WhisperSampleRate
	buf := make([]byte, chunkSamples*2)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			if _, err := io.ReadFull(stdout, buf); err != nil {
				if !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
					fmt.Fprintf(os.Stderr, "\nRead error: %v\n", err)
				}
				return
			}

			samples := pcmToFloat32(buf, chunkSamples)

			ctx, err := model.NewContext()
			if err != nil {
				fmt.Fprintf(os.Stderr, "\nContext error: %v\n", err)
				continue
			}
			ctx.SetThreads(uint(threads))

			if language != "" {
				_ = ctx.SetLanguage(language)
			} else {
				_ = ctx.SetLanguage("auto")
			}

			if err := ctx.Process(samples, nil, nil, nil); err != nil {
				continue
			}

			for {
				seg, err := ctx.NextSegment()
				if errors.Is(err, io.EOF) {
					break
				}
				if err != nil {
					break
				}
				fmt.Print(seg.Text)
				accumulated.WriteString(seg.Text)
			}
		}
	}()

	select {
	case <-sigCh:
	case <-done:
	}

	fmt.Println("\nStopping...")
	_ = cmd.Process.Kill()
	_ = cmd.Wait()

	if text := accumulated.String(); text != "" {
		if err := os.WriteFile("log.txt", []byte(text), 0o644); err != nil {
			return fmt.Errorf("saving log: %w", err)
		}
		fmt.Println("Transcription saved to log.txt")
	}

	return nil
}

func microphoneCommand() (*exec.Cmd, error) {
	ff := findFFmpeg()
	if ff == "" {
		return nil, fmt.Errorf("ffmpeg not found; install ffmpeg for live transcription")
	}

	switch runtime.GOOS {
	case "windows":
		return exec.Command(ff,
			"-loglevel", "quiet",
			"-f", "dshow",
			"-i", "audio=default",
			"-ar", "16000", "-ac", "1",
			"-sample_fmt", "s16", "-f", "s16le", "-",
		), nil
	case "darwin":
		return exec.Command(ff,
			"-loglevel", "quiet",
			"-f", "avfoundation",
			"-i", ":default",
			"-ar", "16000", "-ac", "1",
			"-sample_fmt", "s16", "-f", "s16le", "-",
		), nil
	default:
		return exec.Command(ff,
			"-loglevel", "quiet",
			"-f", "pulse", "-i", "default",
			"-ar", "16000", "-ac", "1",
			"-sample_fmt", "s16", "-f", "s16le", "-",
		), nil
	}
}
