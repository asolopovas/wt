//go:build android

package gui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

func showTranscriptPreview(title string, body fyne.CanvasObject, parent fyne.Window, onClose func()) func() {
	w := fyne.CurrentApp().NewWindow("")
	closeBtn := widget.NewButton("CLOSE", func() {
		w.Close()
		if onClose != nil {
			onClose()
		}
	})
	chrome := container.NewBorder(nil, nil, nil, closeBtn, widget.NewLabel(title))
	w.SetContent(container.NewBorder(chrome, nil, nil, nil, body))
	w.Show()
	return w.Close
}
