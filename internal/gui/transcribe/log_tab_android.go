//go:build android

package transcribe

import "fyne.io/fyne/v2"

func BuildLogTab(tp *Panel) fyne.CanvasObject {
	return buildLogPanel(tp.LogEntry, nil, tp.CopyLogBtn, tp.ClearLogBtn, tp.AutoBtn)
}
