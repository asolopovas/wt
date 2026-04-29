//go:build android

package gui

import (
	"fyne.io/fyne/v2"
)

func showTranscriptPreview(_ string, body fyne.CanvasObject, parent fyne.Window, onClose func()) func() {
	w := fyne.CurrentApp().NewWindow("")
	w.SetOnClosed(func() {
		if onClose != nil {
			onClose()
		}
	})
	w.SetContent(body)
	w.Show()
	return w.Close
}
