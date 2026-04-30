package gui

import (
	"fmt"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

func formatRunDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	total := int(d / time.Second)
	h := total / 3600
	m := (total % 3600) / 60
	s := total % 60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}

func (p *transcribePanel) startRunTimer() {
	p.timerStopMu.Lock()
	defer p.timerStopMu.Unlock()
	if p.timerStop != nil {
		return
	}
	p.runStart = time.Now()
	stop := make(chan struct{})
	p.timerStop = stop
	fyne.Do(func() {
		p.timerText.Text = "0:00"
		p.timerText.Refresh()
	})
	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				txt := formatRunDuration(time.Since(p.runStart))
				fyne.Do(func() {
					p.timerText.Text = txt
					p.timerText.Refresh()
				})
			}
		}
	}()
}

func (p *transcribePanel) stopRunTimer() {
	p.timerStopMu.Lock()
	stop := p.timerStop
	p.timerStop = nil
	p.timerStopMu.Unlock()
	if stop != nil {
		close(stop)
	}
	if !p.runStart.IsZero() {
		final := formatRunDuration(time.Since(p.runStart))
		fyne.Do(func() {
			p.timerText.Text = final
			p.timerText.Refresh()
		})
	}
}

func (p *transcribePanel) setStats(msg string) {
	fyne.Do(func() {
		p.statsLine.SetText(msg)
	})
}

func (p *transcribePanel) setRunning(running bool) {
	p.mu.Lock()
	p.running = running
	p.mu.Unlock()

	if running {
		p.progressTarget.Store(0)
		p.statusTarget.Store(nil)
		p.startSmoothUpdates()
		p.startRunTimer()
		acquireWakeLock()
	} else {
		p.stopSmoothUpdates()
		p.stopRunTimer()
		releaseWakeLock()
	}

	fyne.Do(func() {
		if running {
			p.transcribeBtn.SetText("CANCEL")
			p.transcribeBtn.Importance = widget.DangerImportance
			p.clearBtn.Disable()
			p.clearCacheBtn.Disable()
			p.progress.Show()
			p.progress.SetValue(0)
		} else {
			p.transcribeBtn.SetText("TRANSCRIBE")
			p.transcribeBtn.Importance = widget.HighImportance
			p.clearBtn.Enable()
			p.clearCacheBtn.Enable()
		}
	})
}

func (p *transcribePanel) onCancel() {
	p.cancelled.Store(true)
	p.mu.Lock()
	if p.cancelFunc != nil {
		p.cancelFunc()
	}
	p.mu.Unlock()
	p.appendLog("Cancelling...")
	p.setStatus("Cancelling...")
}

func (p *transcribePanel) toggleAutoScroll() {
	on := !p.autoScroll.Load()
	p.autoScroll.Store(on)
	fyne.Do(func() {
		if on {
			p.autoBtn.Importance = widget.HighImportance
			if p.logEntry != nil {
				text := p.logEntry.Text
				p.logEntry.CursorRow = strings.Count(text, "\n")
				p.logEntry.CursorColumn = len(text) - strings.LastIndex(text, "\n") - 1
				p.logEntry.Refresh()
			}
		} else {
			p.autoBtn.Importance = widget.LowImportance
		}
		p.autoBtn.Refresh()
	})
}

func (p *transcribePanel) debugLog(msg string) {
	if !p.settings.debug() {
		return
	}
	p.appendLog("  [debug] " + msg)
}
