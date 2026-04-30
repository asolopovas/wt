//go:build !android

package transcribe

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func BuildLogTab(tp *Panel) fyne.CanvasObject {
	shareBtn := newPointerButtonWithIcon("", theme.MailForwardIcon(), tp.onShareLog)
	shareBtn.Importance = widget.LowImportance

	return buildLogPanel(tp.LogEntry, tp.StatsLine, tp.CopyLogBtn, tp.ClearLogBtn, tp.AutoBtn, shareBtn)
}
