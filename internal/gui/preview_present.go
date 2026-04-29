//go:build !android

package gui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

func showTranscriptPreview(title string, body fyne.CanvasObject, parent fyne.Window, onClose func()) func() {
	header := widget.NewLabel(title)
	content := container.NewBorder(header, nil, nil, nil, body)
	pop := widget.NewModalPopUp(content, parent.Canvas())
	if size, ok := previewDialogSize(); ok {
		pop.Resize(size)
	}
	pop.Show()
	return func() {
		pop.Hide()
		if onClose != nil {
			onClose()
		}
	}
}
