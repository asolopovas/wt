//go:build android

package preview

import (
	"fyne.io/fyne/v2"

	"github.com/asolopovas/wt/internal/gui/decor"
)

func ScrollMinSize() fyne.Size {
	return fyne.NewSize(300, 220)
}

func DialogSize() (fyne.Size, bool) {
	return fyne.Size{}, false
}

func TopInset() float32    { return decor.SpaceXXL*2 + decor.SpaceSM }
func BottomInset() float32 { return decor.SpaceXXL*3 + decor.SpaceLG }
