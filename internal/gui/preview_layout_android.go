//go:build android

package gui

import "fyne.io/fyne/v2"

func previewScrollMinSize() fyne.Size {
	return fyne.NewSize(300, 220)
}

func previewDialogSize() (fyne.Size, bool) {
	return fyne.Size{}, false
}
