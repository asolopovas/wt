//go:build android

package gui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

func showTranscriptPreview(_ string, body fyne.CanvasObject, parent fyne.Window, onClose func()) func() {
	pop := widget.NewModalPopUp(body, parent.Canvas())
	pop.Resize(parent.Canvas().Size())
	pop.Show()
	return func() {
		pop.Hide()
		if onClose != nil {
			onClose()
		}
	}
}
