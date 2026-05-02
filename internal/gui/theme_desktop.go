//go:build !android

package gui

import (
	"image/color"

	"github.com/asolopovas/wt/internal/gui/decor"
)

var (
	colSecondary     = color.NRGBA{R: 255, G: 208, B: 135, A: 255}
	newSectionHeader = decor.NewSectionHeader
)
