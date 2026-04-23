//go:build !android

package transcriber

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	shared "github.com/asolopovas/wt/internal"
)

var (
	ffmpegOnce sync.Once
	ffmpegPath string
)

func findFFmpeg() string {
	ffmpegOnce.Do(func() {
		if p, err := exec.LookPath("ffmpeg"); err == nil {
			ffmpegPath = p
			return
		}
		if runtime.GOOS != "windows" {
			return
		}
		localAppData := os.Getenv("LOCALAPPDATA")
		userProfile := os.Getenv("USERPROFILE")
		candidates := []string{
			filepath.Join(localAppData, "Microsoft", "WinGet", "Links", "ffmpeg.exe"),
		}
		wingetPkgs := filepath.Join(localAppData, "Microsoft", "WinGet", "Packages")
		if entries, err := os.ReadDir(wingetPkgs); err == nil {
			for _, e := range entries {
				if strings.Contains(e.Name(), "FFmpeg") {
					pkgDir := filepath.Join(wingetPkgs, e.Name())
					if subs, err := os.ReadDir(pkgDir); err == nil {
						for _, s := range subs {
							if s.IsDir() && strings.HasPrefix(s.Name(), "ffmpeg") {
								candidates = append(candidates, filepath.Join(pkgDir, s.Name(), "bin", "ffmpeg.exe"))
							}
						}
					}
				}
			}
		}
		candidates = append(candidates,
			filepath.Join(userProfile, "scoop", "shims", "ffmpeg.exe"),
			`C:\ProgramData\chocolatey\bin\ffmpeg.exe`,
		)
		for _, c := range candidates {
			if _, err := os.Stat(c); err == nil {
				ffmpegPath = c
				return
			}
		}
	})
	return ffmpegPath
}

func LoadAudioSamples(path string) ([]float32, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("audio file not found: %w", err)
	}

	if strings.EqualFold(filepath.Ext(path), ".wav") {
		return loadWAV(path)
	}

	return convertAndLoad(path)
}

func loadWAV(path string) ([]float32, error) {
	samples, err := readPCM16WAV(path)
	if err == nil {
		return samples, nil
	}
	return convertAndLoad(path)
}

func convertAndLoad(path string) ([]float32, error) {
	if findFFmpeg() == "" {
		return nil, fmt.Errorf("ffmpeg not found (needed to convert %s); install ffmpeg or provide a 16kHz mono WAV file", filepath.Ext(path))
	}

	cacheDir := shared.CacheDir()
	cacheFile, err := AudioCacheKey(path)
	if err != nil {
		return convertToTemp(path)
	}

	cachePath := filepath.Join(cacheDir, cacheFile)
	if _, err := os.Stat(cachePath); err == nil {
		return readPCM16WAV(cachePath)
	}

	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return convertToTemp(path)
	}

	if err := runFFmpeg(path, cachePath); err != nil {
		_ = os.Remove(cachePath)
		return nil, fmt.Errorf("ffmpeg conversion failed: %w", err)
	}

	return readPCM16WAV(cachePath)
}

func convertToTemp(path string) ([]float32, error) {
	tmpFile, err := os.CreateTemp("", "wt-*.wav")
	if err != nil {
		return nil, fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	_ = tmpFile.Close()
	defer func() {
		_ = os.Remove(tmpPath)
	}()

	if err := runFFmpeg(path, tmpPath); err != nil {
		return nil, fmt.Errorf("ffmpeg conversion failed: %w", err)
	}

	return readPCM16WAV(tmpPath)
}

func runFFmpeg(input, output string) error {
	cmd := exec.Command(findFFmpeg(),
		"-loglevel", "error",
		"-i", input,
		"-ar", "16000",
		"-ac", "1",
		"-sample_fmt", "s16",
		"-f", "wav",
		"-y",
		output,
	)
	shared.HideWindow(cmd)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if stderr.Len() > 0 {
			return fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
		}
		return err
	}
	return nil
}
