//go:build !android

package preview

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"

	"github.com/asolopovas/wt/internal/gui/decor"
)

func ShowTranscript(_ string, body fyne.CanvasObject, parent fyne.Window, onClose func()) func() {
	pop := widget.NewModalPopUp(decor.DialogBordered(body), parent.Canvas())
	if size, ok := DialogSize(); ok {
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
