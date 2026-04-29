//go:build !android

package gui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
)

func showTranscriptPreview(title string, body fyne.CanvasObject, parent fyne.Window) func() {
	dlg := dialog.NewCustom(title, "Close", body, parent)
	if size, ok := previewDialogSize(); ok {
		dlg.Resize(size)
	}
	dlg.Show()
	return dlg.Hide
}
