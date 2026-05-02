//go:build !android

package player

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	shared "github.com/asolopovas/wt/internal"
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

// Player wraps an ffplay subprocess. Pause/Seek are implemented by killing and
// restarting at a new offset (ffplay has no in-band control without a TTY).
type Player struct {
	mu        sync.Mutex
	cmd       *exec.Cmd
	key       string
	path      string
	onStop    func(key string)
	startWall time.Time
	startSec  float64 // playback offset (seconds) at process start
	endSec    float64 // 0 = play to end; otherwise stop after this absolute pos
	stopping  bool
}

func (p *Player) Playing(key string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.cmd != nil && p.key == key
}

func (p *Player) IsPlaying() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.cmd != nil
}

// Position returns the current playback position in seconds, 0 if stopped.
func (p *Player) Position() float64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cmd == nil {
		return 0
	}
	return p.startSec + time.Since(p.startWall).Seconds()
}

// Start plays the whole file from the beginning. Back-compat.
func (p *Player) Start(key, path string, onStop func(key string)) error {
	return p.StartRange(key, path, 0, 0, onStop)
}

// StartRange plays [startSec, endSec). endSec<=0 means play to EOF.
func (p *Player) StartRange(key, path string, startSec, endSec float64, onStop func(key string)) error {
	p.Stop()
	bin, err := findFfplay()
	if err != nil {
		return err
	}
	args := []string{"-nodisp", "-autoexit", "-loglevel", "quiet"}
	if startSec > 0 {
		args = append(args, "-ss", formatSec(startSec))
	}
	if endSec > startSec && endSec > 0 {
		args = append(args, "-t", formatSec(endSec-startSec))
	}
	args = append(args, path)
	cmd := exec.Command(bin, args...)
	shared.HideWindow(cmd)
	if err := cmd.Start(); err != nil {
		return err
	}
	p.mu.Lock()
	p.cmd = cmd
	p.key = key
	p.path = path
	p.onStop = onStop
	p.startWall = time.Now()
	p.startSec = startSec
	p.endSec = endSec
	p.stopping = false
	p.mu.Unlock()

	go func() {
		_ = cmd.Wait()
		p.mu.Lock()
		stoppedKey := p.key
		stoppedCb := p.onStop
		stopping := p.stopping
		if p.cmd == cmd {
			p.cmd = nil
			p.key = ""
			p.onStop = nil
			p.path = ""
		}
		p.mu.Unlock()
		// Don't fire onStop when caller explicitly stopped (avoids icon flicker
		// during seek/restart which itself calls Stop).
		if !stopping && stoppedCb != nil {
			stoppedCb(stoppedKey)
		}
	}()
	return nil
}

func (p *Player) Stop() {
	p.mu.Lock()
	cmd := p.cmd
	p.stopping = true
	p.mu.Unlock()
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	}
}

func formatSec(s float64) string {
	if s < 0 {
		s = 0
	}
	// ffmpeg accepts plain seconds with decimals
	whole := int(s)
	frac := int((s - float64(whole)) * 1000)
	return itoaPad(whole, 1) + "." + itoaPad(frac, 3)
}

func itoaPad(n, width int) string {
	if n < 0 {
		n = 0
	}
	s := []byte{}
	if n == 0 {
		s = []byte{'0'}
	}
	for n > 0 {
		s = append([]byte{byte('0' + n%10)}, s...)
		n /= 10
	}
	for len(s) < width {
		s = append([]byte{'0'}, s...)
	}
	return string(s)
}
