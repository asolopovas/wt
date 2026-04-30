//go:build !android

package transcriber

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"

	shared "github.com/asolopovas/wt/internal"
)

var (
	ffmpegOnce  sync.Once
	ffmpegPath  string
	ffprobeOnce sync.Once
	ffprobePath string
)

func findFFprobe() string {
	ffprobeOnce.Do(func() {
		if p, err := exec.LookPath("ffprobe"); err == nil {
			ffprobePath = p
			return
		}
		ff := findFFmpeg()
		if ff == "" {
			return
		}
		dir := filepath.Dir(ff)
		candidate := filepath.Join(dir, "ffprobe")
		if runtime.GOOS == "windows" {
			candidate += ".exe"
		}
		if _, err := os.Stat(candidate); err == nil {
			ffprobePath = candidate
		}
	})
	return ffprobePath
}

func ProbeDurationMs(path string) int64 {
	if probe := findFFprobe(); probe != "" {
		cmd := exec.Command(probe,
			"-v", "error",
			"-show_entries", "format=duration",
			"-of", "default=noprint_wrappers=1:nokey=1",
			path,
		)
		shared.HideWindow(cmd)
		out, err := cmd.Output()
		if err == nil {
			s := strings.TrimSpace(string(out))
			if sec, err := strconv.ParseFloat(s, 64); err == nil && sec > 0 {
				return int64(sec * 1000)
			}
		}
	}
	if ff := findFFmpeg(); ff != "" {
		cmd := exec.Command(ff, "-i", path)
		shared.HideWindow(cmd)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		_ = cmd.Run()
		if ms := parseFFmpegDuration(stderr.String()); ms > 0 {
			return ms
		}
	}
	return 0
}

func parseFFmpegDuration(stderr string) int64 {
	idx := strings.Index(stderr, "Duration:")
	if idx < 0 {
		return 0
	}
	rest := stderr[idx+len("Duration:"):]
	end := strings.IndexByte(rest, ',')
	if end < 0 {
		return 0
	}
	hms := strings.TrimSpace(rest[:end])
	parts := strings.Split(hms, ":")
	if len(parts) != 3 {
		return 0
	}
	h, err1 := strconv.Atoi(parts[0])
	m, err2 := strconv.Atoi(parts[1])
	s, err3 := strconv.ParseFloat(parts[2], 64)
	if err1 != nil || err2 != nil || err3 != nil {
		return 0
	}
	total := float64(h*3600+m*60) + s
	if total <= 0 {
		return 0
	}
	return int64(total * 1000)
}

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
