//go:build android

package gui

import "fyne.io/fyne/v2"

func previewScrollMinSize() fyne.Size {
	return fyne.NewSize(300, 220)
}

func previewDialogSize() (fyne.Size, bool) {
	return fyne.Size{}, false
}

func previewTopInset() float32    { return spaceXXL*2 + spaceSM }
func previewBottomInset() float32 { return spaceXXL*3 + spaceLG }
