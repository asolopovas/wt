//go:build android

package gui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

func showTranscriptPreview(_ string, body fyne.CanvasObject, parent fyne.Window, onClose func()) func() {
	pop := widget.NewModalPopUp(dialogBordered(body), parent.Canvas())
	cs := parent.Canvas().Size()
	const margin = 24
	w := cs.Width - margin*2
	h := cs.Height - margin*2
	if w < 1 {
		w = cs.Width
	}
	if h < 1 {
		h = cs.Height
	}
	pop.Resize(fyne.NewSize(w, h))
	pop.Show()
	return func() {
		pop.Hide()
		if onClose != nil {
			onClose()
		}
	}
}
