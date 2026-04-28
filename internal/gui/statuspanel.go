package gui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

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
	} else {
		p.stopSmoothUpdates()
	}

	fyne.Do(func() {
		if running {
			p.transcribeBtn.SetText("CANCEL")
			p.transcribeBtn.Importance = widget.DangerImportance
			p.clearBtn.Disable()
			p.clearCacheBtn.Disable()
			if p.previewBtn != nil {
				p.previewBtn.Disable()
			}
			p.progress.Show()
			p.progress.SetValue(0)
		} else {
			p.transcribeBtn.SetText("TRANSCRIBE")
			p.transcribeBtn.Importance = widget.HighImportance
			p.clearBtn.Enable()
			p.clearCacheBtn.Enable()
			if p.previewBtn != nil && len(p.results) > 0 {
				p.previewBtn.Enable()
			}
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
			p.logScroll.ScrollToBottom()
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
