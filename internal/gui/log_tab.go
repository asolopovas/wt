//go:build !android

package gui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func buildLogTab(tp *transcribePanel) fyne.CanvasObject {
	shareBtn := newPointerButtonWithIcon("", theme.MailForwardIcon(), tp.onShareLog)
	shareBtn.Importance = widget.LowImportance

	return buildLogPanel(tp.logEntry, tp.statsLine, tp.copyLogBtn, tp.clearLogBtn, tp.autoBtn, shareBtn)
}
