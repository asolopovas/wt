//go:build !android

package gui

import (
	"fmt"
	"math"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
)

func setupTray(a fyne.App, w fyne.Window, tp *transcribePanel) {
	dApp, ok := a.(desktop.App)
	if !ok {
		return
	}

	dApp.SetSystemTrayIcon(appIcon)

	showItem := fyne.NewMenuItem("Show wt", func() {
		w.Show()
		w.RequestFocus()
	})
	cancelItem := fyne.NewMenuItem("Cancel transcription", func() {
		tp.mu.Lock()
		running := tp.running
		tp.mu.Unlock()
		if running {
			tp.onCancel()
		}
	})

	menu := fyne.NewMenu("wt", showItem, cancelItem)
	dApp.SetSystemTrayMenu(menu)

	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		var lastLabel string
		for range ticker.C {
			label := trayProgressLabel(tp)
			if label == lastLabel {
				continue
			}
			lastLabel = label
			fyne.Do(func() {
				showItem.Label = label
				dApp.SetSystemTrayMenu(menu)
			})
		}
	}()
}

func trayProgressLabel(tp *transcribePanel) string {
	tp.mu.Lock()
	running := tp.running
	tp.mu.Unlock()
	if !running {
		return "Show wt"
	}
	pct := math.Float64frombits(tp.progressTarget.Load()) * 100
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	return fmt.Sprintf("Show wt — %.0f%%", pct)
}
