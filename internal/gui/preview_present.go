//go:build !android

package gui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

func showTranscriptPreview(_ string, body fyne.CanvasObject, parent fyne.Window, onClose func()) func() {
	pop := widget.NewModalPopUp(body, parent.Canvas())
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
