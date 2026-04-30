//go:build android

package gui

import "fyne.io/fyne/v2"

func buildLogTab(tp *transcribePanel) fyne.CanvasObject {
	return buildLogPanel(tp.logEntry, nil, tp.copyLogBtn, tp.clearLogBtn, tp.autoBtn)
}
