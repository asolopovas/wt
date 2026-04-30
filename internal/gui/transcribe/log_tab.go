package transcribe

import (
	"runtime"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/asolopovas/wt/internal/gui/decor"
)

func BuildLogTab(tp *Panel) fyne.CanvasObject {
	var panel fyne.CanvasObject
	if runtime.GOOS == "android" {
		panel = buildLogPanel(tp.LogEntry, nil, tp.CopyLogBtn, tp.ClearLogBtn, tp.AutoBtn)
	} else {
		shareBtn := newPointerButtonWithIcon("", theme.MailForwardIcon(), tp.onShareLog)
		shareBtn.Importance = widget.LowImportance
		panel = buildLogPanel(tp.LogEntry, nil, tp.CopyLogBtn, tp.ClearLogBtn, tp.AutoBtn, shareBtn)
	}
	footer := container.NewPadded(container.NewCenter(tp.StatsFooter))
	return container.NewBorder(nil, container.NewVBox(footer, decor.VGap(spaceLG)), nil, nil, panel)
}
