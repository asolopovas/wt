package preview

import (
	"runtime"

	"fyne.io/fyne/v2"
)

func ScrollMinSize() fyne.Size {
	if runtime.GOOS == "android" {
		return fyne.NewSize(300, 220)
	}
	return fyne.NewSize(640, 280)
}

func DialogSize() (fyne.Size, bool) {
	if runtime.GOOS == "android" {
		return fyne.Size{}, false
	}
	return fyne.NewSize(760, 460), true
}

func TopInset() float32    { return 0 }
func BottomInset() float32 { return 0 }
