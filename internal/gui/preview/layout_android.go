//go:build android

package preview

import (
	"fyne.io/fyne/v2"
)

func ScrollMinSize() fyne.Size {
	return fyne.NewSize(300, 220)
}

func DialogSize() (fyne.Size, bool) {
	return fyne.Size{}, false
}

func TopInset() float32    { return 0 }
func BottomInset() float32 { return 0 }
