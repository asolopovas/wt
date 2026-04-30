package transcribe

import (
	"runtime"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func BuildLogTab(tp *Panel) fyne.CanvasObject {
	if runtime.GOOS == "android" {
		return buildLogPanel(tp.LogEntry, nil, tp.CopyLogBtn, tp.ClearLogBtn, tp.AutoBtn)
	}
	shareBtn := newPointerButtonWithIcon("", theme.MailForwardIcon(), tp.onShareLog)
	shareBtn.Importance = widget.LowImportance
	return buildLogPanel(tp.LogEntry, tp.StatsLine, tp.CopyLogBtn, tp.ClearLogBtn, tp.AutoBtn, shareBtn)
}
