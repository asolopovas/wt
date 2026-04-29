//go:build android

package gui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

func showTranscriptPreview(title string, body fyne.CanvasObject, parent fyne.Window, onClose func()) func() {
	w := fyne.CurrentApp().NewWindow("")
	w.SetOnClosed(func() {
		if onClose != nil {
			onClose()
		}
	})
	chrome := container.NewPadded(widget.NewLabel(title))
	w.SetContent(container.NewBorder(chrome, nil, nil, nil, body))
	w.Show()
	return w.Close
}
