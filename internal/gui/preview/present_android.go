//go:build android

package preview

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"

	"github.com/asolopovas/wt/internal/gui/decor"
)

func ShowTranscript(_ string, body fyne.CanvasObject, parent fyne.Window, onClose func()) func() {
	pop := widget.NewModalPopUp(decor.DialogBordered(body), parent.Canvas())
	cs := parent.Canvas().Size()
	const sideMargin = 16
	w := cs.Width - sideMargin*2
	if w < 1 {
		w = cs.Width
	}
	h := cs.Height * 0.7
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
