//go:build !android

package gui

import "fyne.io/fyne/v2"

func previewScrollMinSize() fyne.Size {
	return fyne.NewSize(640, 360)
}

func previewDialogSize() (fyne.Size, bool) {
	return fyne.NewSize(760, 600), true
}
