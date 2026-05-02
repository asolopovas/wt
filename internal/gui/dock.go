package gui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	shared "github.com/asolopovas/wt/internal"
	"github.com/asolopovas/wt/internal/gui/player"
	"github.com/asolopovas/wt/internal/gui/transcribe"
	"github.com/asolopovas/wt/internal/gui/waveform"
	"github.com/asolopovas/wt/internal/transcriber"
)

type playerDock struct {
	window     fyne.Window
	transcribe *transcribe.Panel

	wave       *waveform.Widget
	playBtn    *pointerButton
	stopBtn    *pointerButton
	saveBtn    *pointerButton
	rerunBtn   *pointerButton
	closeBtn   *pointerButton
	titleLbl   *canvas.Text

	root       *fyne.Container
	body       *fyne.Container

	mu         sync.Mutex
	currentKey string
	currentPath string
	duration   float64
	loadingCtx context.CancelFunc

	pl       player.Player
	tickStop chan struct{}
}

func newPlayerDock(window fyne.Window, tp *transcribe.Panel) *playerDock {
	d := &playerDock{window: window, transcribe: tp}
	d.build()
	return d
}

func (d *playerDock) build() {
	d.wave = waveform.New()
	d.wave.OnSeek = d.onSeek
	d.wave.OnRegionChanged = func(s, e float64) { /* live-update labels only */ }

	d.playBtn = newPointerButtonWithIcon("", theme.MediaPlayIcon(), nil)
	d.playBtn.Importance = widget.LowImportance
	d.playBtn.OnTapped = d.onPlayPause

	d.stopBtn = newPointerButtonWithIcon("", theme.MediaStopIcon(), nil)
	d.stopBtn.Importance = widget.LowImportance
	d.stopBtn.OnTapped = d.onStop

	d.saveBtn = newPointerButtonWithIcon("", theme.DocumentSaveIcon(), nil)
	d.saveBtn.Importance = widget.LowImportance
	d.saveBtn.OnTapped = d.onSaveTrim

	d.rerunBtn = newPointerButtonWithIcon("", theme.MediaReplayIcon(), nil)
	d.rerunBtn.Importance = widget.LowImportance
	d.rerunBtn.OnTapped = d.onTranscribeTrim

	d.closeBtn = newPointerButtonWithIcon("", theme.CancelIcon(), nil)
	d.closeBtn.Importance = widget.LowImportance
	d.closeBtn.OnTapped = d.Close

	d.titleLbl = canvas.NewText("", colMuted)
	d.titleLbl.TextSize = textBody
	d.titleLbl.TextStyle = fyne.TextStyle{Bold: true}

	wrap := func(b fyne.CanvasObject) fyne.CanvasObject {
		return container.NewGridWrap(fyne.NewSize(32, 32), b)
	}
	transport := container.NewHBox(
		wrap(d.playBtn), wrap(d.stopBtn),
		newSectionDivider(),
		wrap(d.saveBtn), wrap(d.rerunBtn),
	)

	header := container.NewBorder(nil, nil, d.titleLbl, wrap(d.closeBtn))

	d.body = container.NewBorder(
		header,
		container.NewPadded(transport),
		nil, nil,
		container.NewPadded(d.wave),
	)

	bg := newPanelBackground()
	d.root = container.NewStack(bg, container.NewPadded(d.body))
	d.root.Hide()
}

// Container returns the dock root for layout mounting.
func (d *playerDock) Container() fyne.CanvasObject { return d.root }

// Load opens a file in the dock and starts loading peaks. Auto-plays from start.
func (d *playerDock) Load(key, path, displayName string, autoplay bool) {
	d.mu.Lock()
	if d.loadingCtx != nil {
		d.loadingCtx()
	}
	d.currentKey = key
	d.currentPath = path
	d.duration = 0
	ctx, cancel := context.WithCancel(context.Background())
	d.loadingCtx = cancel
	d.mu.Unlock()

	d.pl.Stop()
	d.stopTick()
	d.titleLbl.Text = displayName
	d.titleLbl.Refresh()
	d.wave.SetRegion(0, 1)
	d.wave.SetPlayhead(-1)
	d.wave.SetLoading(true)
	d.setPlayIcon(false)
	d.root.Show()
	d.root.Refresh()

	go func() {
		p, err := waveform.Extract(path)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			fyne.Do(func() {
				d.wave.SetLoading(false)
				showError(d.window, fmt.Errorf("waveform: %w", err))
			})
			return
		}
		fyne.Do(func() {
			d.mu.Lock()
			d.duration = p.Duration
			d.mu.Unlock()
			d.wave.SetPeaks(p)
			if autoplay {
				d.startPlayback()
			}
		})
	}()
}

func (d *playerDock) Close() {
	d.pl.Stop()
	d.stopTick()
	d.mu.Lock()
	if d.loadingCtx != nil {
		d.loadingCtx()
		d.loadingCtx = nil
	}
	d.currentKey = ""
	d.currentPath = ""
	d.duration = 0
	d.mu.Unlock()
	d.root.Hide()
	d.root.Refresh()
}

func (d *playerDock) onPlayPause() {
	if d.pl.IsPlaying() {
		d.pl.Stop()
		d.stopTick()
		d.setPlayIcon(false)
		return
	}
	d.startPlayback()
}

func (d *playerDock) onStop() {
	d.pl.Stop()
	d.stopTick()
	d.setPlayIcon(false)
	d.wave.SetPlayhead(-1)
}

func (d *playerDock) startPlayback() {
	d.mu.Lock()
	path := d.currentPath
	key := d.currentKey
	dur := d.duration
	d.mu.Unlock()
	if path == "" || dur <= 0 {
		return
	}
	rs, re := d.wave.Region()
	startSec := rs * dur
	endSec := re * dur
	err := d.pl.StartRange(key, path, startSec, endSec, func(string) {
		fyne.Do(func() {
			d.setPlayIcon(false)
			d.wave.SetPlayhead(-1)
			d.stopTick()
		})
	})
	if err != nil {
		showError(d.window, fmt.Errorf("ffplay not available: %w", err))
		return
	}
	d.setPlayIcon(true)
	d.startTick(startSec, endSec, dur)
}

func (d *playerDock) onSeek(frac float64) {
	d.mu.Lock()
	dur := d.duration
	path := d.currentPath
	key := d.currentKey
	d.mu.Unlock()
	if dur <= 0 || path == "" {
		return
	}
	_, re := d.wave.Region()
	endSec := re * dur
	startSec := frac * dur
	if startSec >= endSec {
		return
	}
	wasPlaying := d.pl.IsPlaying()
	d.pl.Stop()
	d.stopTick()
	d.wave.SetPlayhead(frac)
	if wasPlaying {
		err := d.pl.StartRange(key, path, startSec, endSec, func(string) {
			fyne.Do(func() {
				d.setPlayIcon(false)
				d.wave.SetPlayhead(-1)
				d.stopTick()
			})
		})
		if err == nil {
			d.setPlayIcon(true)
			d.startTick(startSec, endSec, dur)
		}
	}
}

func (d *playerDock) startTick(startSec, endSec, dur float64) {
	d.stopTick()
	stop := make(chan struct{})
	d.tickStop = stop
	go func() {
		t := time.NewTicker(80 * time.Millisecond)
		defer t.Stop()
		for {
			select {
			case <-stop:
				return
			case <-t.C:
				pos := d.pl.Position()
				if pos >= endSec {
					pos = endSec
				}
				if dur <= 0 {
					continue
				}
				frac := pos / dur
				if frac < 0 {
					frac = 0
				}
				if frac > 1 {
					frac = 1
				}
				fyne.Do(func() { d.wave.SetPlayhead(frac) })
			}
		}
	}()
}

func (d *playerDock) stopTick() {
	if d.tickStop != nil {
		close(d.tickStop)
		d.tickStop = nil
	}
}

func (d *playerDock) setPlayIcon(playing bool) {
	if playing {
		d.playBtn.SetIcon(theme.MediaPauseIcon())
	} else {
		d.playBtn.SetIcon(theme.MediaPlayIcon())
	}
}

func (d *playerDock) onSaveTrim() {
	d.mu.Lock()
	path := d.currentPath
	dur := d.duration
	d.mu.Unlock()
	if path == "" || dur <= 0 {
		return
	}
	rs, re := d.wave.Region()
	startSec := rs * dur
	endSec := re * dur
	if endSec <= startSec {
		return
	}
	ext := filepath.Ext(path)
	base := strings.TrimSuffix(filepath.Base(path), ext)
	suggested := fmt.Sprintf("%s_trim_%s-%s%s", base, secTag(startSec), secTag(endSec), ext)
	d.saveTrimAs(path, startSec, endSec, suggested)
}

func (d *playerDock) saveTrimAs(srcPath string, startSec, endSec float64, suggestedName string) {
	save := dialog.NewFileSave(func(uc fyne.URIWriteCloser, err error) {
		if err != nil || uc == nil {
			return
		}
		out := uc.URI().Path()
		_ = uc.Close()
		if err := os.Remove(out); err != nil && !os.IsNotExist(err) {
			showError(d.window, err)
			return
		}
		bin := transcriber.FindFFmpeg()
		if bin == "" {
			showError(d.window, fmt.Errorf("ffmpeg not found"))
			return
		}
		cmd := exec.Command(bin,
			"-hide_banner", "-loglevel", "error", "-y", "-nostdin",
			"-ss", fmt.Sprintf("%.3f", startSec),
			"-to", fmt.Sprintf("%.3f", endSec),
			"-i", srcPath,
			out,
		)
		shared.HideWindow(cmd)
		if err := cmd.Run(); err != nil {
			showError(d.window, fmt.Errorf("ffmpeg trim: %w", err))
			_ = os.Remove(out)
			return
		}
		showNotice(d.window, notifySuccess, "Saved trim", filepath.Base(out))
	}, d.window)
	save.SetFileName(suggestedName)
	if dir, err := storage.ListerForURI(storage.NewFileURI(filepath.Dir(srcPath))); err == nil {
		save.SetLocation(dir)
	}
	save.Show()
}

func (d *playerDock) onTranscribeTrim() {
	d.mu.Lock()
	path := d.currentPath
	dur := d.duration
	d.mu.Unlock()
	if path == "" || dur <= 0 {
		return
	}
	rs, re := d.wave.Region()
	startSec := rs * dur
	endSec := re * dur
	if endSec <= startSec {
		return
	}
	ext := filepath.Ext(path)
	base := strings.TrimSuffix(filepath.Base(path), ext)
	tmpDir := filepath.Join(shared.CacheDir(), "trims")
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		showError(d.window, err)
		return
	}
	out := filepath.Join(tmpDir, fmt.Sprintf("%s_trim_%s-%s%s", base, secTag(startSec), secTag(endSec), ext))

	bin := transcriber.FindFFmpeg()
	if bin == "" {
		showError(d.window, fmt.Errorf("ffmpeg not found"))
		return
	}
	cmd := exec.Command(bin,
		"-hide_banner", "-loglevel", "error", "-y", "-nostdin",
		"-ss", fmt.Sprintf("%.3f", startSec),
		"-to", fmt.Sprintf("%.3f", endSec),
		"-i", path,
		out,
	)
	shared.HideWindow(cmd)
	if err := cmd.Run(); err != nil {
		showError(d.window, fmt.Errorf("ffmpeg trim: %w", err))
		_ = os.Remove(out)
		return
	}
	d.transcribe.StartTranscription([]string{out})
}

func secTag(s float64) string {
	ms := int(s * 1000)
	return fmt.Sprintf("%d", ms)
}
