package preview

import (
	"runtime"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"

	"github.com/asolopovas/wt/internal/gui/decor"
)

func ShowTranscript(_ string, body fyne.CanvasObject, parent fyne.Window, onClose func()) func() {
	pop := widget.NewModalPopUp(decor.DialogBordered(body), parent.Canvas())
	pop.Resize(transcriptSize(parent.Canvas().Size()))
	pop.Show()
	return func() {
		pop.Hide()
		if onClose != nil {
			onClose()
		}
	}
}

func transcriptSize(canvas fyne.Size) fyne.Size {
	if runtime.GOOS == "android" {
		const sideMargin = 16
		w := canvas.Width - sideMargin*2
		if w < 1 {
			w = canvas.Width
		}
		h := canvas.Height * 0.7
		if h < 1 {
			h = canvas.Height
		}
		return fyne.NewSize(w, h)
	}
	if size, ok := DialogSize(); ok {
		return size
	}
	return canvas
}
