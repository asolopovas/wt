package transcribe

import (
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"

	"github.com/asolopovas/wt/internal/gui/platsvc"
	"github.com/asolopovas/wt/internal/transcriber"
)

func (p *Panel) startRunTimer() {
	p.timerStopMu.Lock()
	defer p.timerStopMu.Unlock()
	if p.timerStop != nil {
		return
	}
	p.runStart = time.Now()
	stop := make(chan struct{})
	p.timerStop = stop
	fyne.Do(func() {
		p.TimerText.Text = "0:00"
		p.TimerText.Refresh()
	})
	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				txt := transcriber.FormatHMS(time.Since(p.runStart))
				fyne.Do(func() {
					p.TimerText.Text = txt
					p.TimerText.Refresh()
				})
			}
		}
	}()
}

func (p *Panel) stopRunTimer() {
	p.timerStopMu.Lock()
	stop := p.timerStop
	p.timerStop = nil
	p.timerStopMu.Unlock()
	if stop != nil {
		close(stop)
	}
	if !p.runStart.IsZero() {
		final := transcriber.FormatHMS(time.Since(p.runStart))
		fyne.Do(func() {
			p.TimerText.Text = final
			p.TimerText.Refresh()
		})
	}
}

func (p *Panel) setStats(msg string) {
	fyne.Do(func() {
		p.StatsLine.SetText(msg)
	})
}

func (p *Panel) setRunning(running bool) {
	p.mu.Lock()
	p.running = running
	p.mu.Unlock()

	if running {
		p.progressTarget.Store(0)
		p.statusTarget.Store(nil)
		p.startSmoothUpdates()
		p.startRunTimer()
		platsvc.AcquireWakeLock()
	} else {
		p.stopSmoothUpdates()
		p.stopRunTimer()
		platsvc.ReleaseWakeLock()
	}

	fyne.Do(func() {
		if running {
			p.TranscribeBtn.SetText("CANCEL")
			p.TranscribeBtn.Importance = widget.DangerImportance
			p.clearBtn.Disable()
			p.clearCacheBtn.Disable()
			p.Progress.Show()
			p.Progress.SetValue(0)
		} else {
			p.TranscribeBtn.SetText("TRANSCRIBE")
			p.TranscribeBtn.Importance = widget.HighImportance
			p.clearBtn.Enable()
			p.clearCacheBtn.Enable()
		}
	})
}

func (p *Panel) OnCancel() {
	p.cancelled.Store(true)
	p.mu.Lock()
	if p.cancelFunc != nil {
		p.cancelFunc()
	}
	p.mu.Unlock()
	p.AppendLog("Cancelling...")
	p.setStatus("Cancelling...")
}

func (p *Panel) toggleAutoScroll() {
	on := !p.autoScroll.Load()
	p.autoScroll.Store(on)
	fyne.Do(func() {
		if on {
			p.AutoBtn.Importance = widget.HighImportance
			if p.LogEntry != nil {
				text := p.LogEntry.Text
				p.LogEntry.CursorRow = strings.Count(text, "\n")
				p.LogEntry.CursorColumn = len(text) - strings.LastIndex(text, "\n") - 1
				p.LogEntry.Refresh()
			}
		} else {
			p.AutoBtn.Importance = widget.LowImportance
		}
		p.AutoBtn.Refresh()
	})
}

func (p *Panel) debugLog(msg string) {
	if !p.Settings.Debug() {
		return
	}
	p.AppendLog("  [debug] " + msg)
}
