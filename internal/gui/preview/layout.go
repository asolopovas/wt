//go:build !android

package preview

import "fyne.io/fyne/v2"

func ScrollMinSize() fyne.Size {
	return fyne.NewSize(640, 360)
}

func DialogSize() (fyne.Size, bool) {
	return fyne.NewSize(760, 600), true
}

func TopInset() float32    { return 0 }
func BottomInset() float32 { return 0 }
