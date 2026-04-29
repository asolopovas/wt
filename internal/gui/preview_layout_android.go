//go:build android

package gui

import "fyne.io/fyne/v2"

func previewScrollMinSize() fyne.Size {
	return fyne.NewSize(300, 220)
}

func previewDialogSize() (fyne.Size, bool) {
	return fyne.Size{}, false
}

func previewTopInset() float32    { return 36 }
func previewBottomInset() float32 { return 56 }
