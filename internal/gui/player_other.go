//go:build !android

package gui

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
)

func findFfplay() (string, error) {
	if p, err := exec.LookPath("ffplay"); err == nil {
		return p, nil
	}
	exe, _ := os.Executable()
	candidates := []string{}
	if exe != "" {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), "ffplay"+exeExt()))
	}
	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates,
			filepath.Join(home, "AppData", "Local", "Microsoft", "WinGet", "Links", "ffplay"+exeExt()),
			filepath.Join(home, "scoop", "shims", "ffplay"+exeExt()),
		)
	}
	candidates = append(candidates,
		`C:\Program Files\ffmpeg\bin\ffplay.exe`,
		`C:\ffmpeg\bin\ffplay.exe`,
		`C:\msys64\mingw64\bin\ffplay.exe`,
	)
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c, nil
		}
	}
	return "", exec.ErrNotFound
}

func exeExt() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
}

type audioPlayer struct {
	mu     sync.Mutex
	cmd    *exec.Cmd
	key    string
	onStop func(key string)
}

func (p *audioPlayer) playing(key string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.cmd != nil && p.key == key
}

func (p *audioPlayer) start(key, path string, onStop func(key string)) error {
	p.stop()
	bin, err := findFfplay()
	if err != nil {
		return err
	}
	cmd := exec.Command(bin, "-nodisp", "-autoexit", "-loglevel", "quiet", path)
	if err := cmd.Start(); err != nil {
		return err
	}
	p.mu.Lock()
	p.cmd = cmd
	p.key = key
	p.onStop = onStop
	p.mu.Unlock()
	go func() {
		_ = cmd.Wait()
		p.mu.Lock()
		stoppedKey := p.key
		stoppedCb := p.onStop
		if p.cmd == cmd {
			p.cmd = nil
			p.key = ""
			p.onStop = nil
		}
		p.mu.Unlock()
		if stoppedCb != nil {
			stoppedCb(stoppedKey)
		}
	}()
	return nil
}

func (p *audioPlayer) stop() {
	p.mu.Lock()
	cmd := p.cmd
	stoppedKey := p.key
	stoppedCb := p.onStop
	p.cmd = nil
	p.key = ""
	p.onStop = nil
	p.mu.Unlock()
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
	if stoppedCb != nil && cmd != nil {
		stoppedCb(stoppedKey)
	}
}
