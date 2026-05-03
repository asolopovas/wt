package transcribe

import (
	"fmt"
	"runtime"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"

	shared "github.com/asolopovas/wt/internal"
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
		if p.TimerSep != nil {
			p.TimerSep.Show()
		}
	})
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		last := "0:00"
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				txt := transcriber.FormatHMS(time.Since(p.runStart))
				if txt == last {
					continue
				}
				last = txt
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

func (p *Panel) setRunning(running bool) {
	p.mu.Lock()
	p.running = running
	p.mu.Unlock()

	if running {
		p.progressTarget.Store(0)
		p.statusTarget.Store(nil)
		p.startSmoothUpdates()
		p.startRunTimer()
		platsvc.StartForegroundService()
		platsvc.AcquireWakeLock()
		platsvc.KeepScreenOn()
	} else {
		p.stopSmoothUpdates()
		p.stopRunTimer()
		platsvc.ReleaseWakeLock()
		platsvc.ReleaseScreenOn()
		platsvc.StopForegroundService()
	}

	fyne.Do(func() {
		if running {
			p.TranscribeBtn.SetText("CANCEL")
			p.TranscribeBtn.Importance = widget.DangerImportance
			p.clearBtn.Disable()
			p.clearCacheBtn.Disable()
			if p.CancelBtn != nil {
				p.CancelBtn.Enable()
			}
			p.Progress.Show()
			p.Progress.SetValue(0)
		} else {
			p.TranscribeBtn.SetText("TRANSCRIBE")
			p.TranscribeBtn.Importance = widget.HighImportance
			p.clearBtn.Enable()
			p.clearCacheBtn.Enable()
			if p.CancelBtn != nil {
				p.CancelBtn.Disable()
			}
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
	if _, file, line, ok := runtime.Caller(1); ok {
		short := file
		if idx := strings.LastIndexByte(file, '/'); idx >= 0 {
			if idx2 := strings.LastIndexByte(file[:idx], '/'); idx2 >= 0 {
				short = file[idx2+1:]
			} else {
				short = file[idx+1:]
			}
		}
		msg = fmt.Sprintf("[%s:%d] %s", short, line, msg)
	}
	shared.LogDebug(msg)
}
