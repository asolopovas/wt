package decor

import "image/color"

var Transparent = color.NRGBA{}

var (
	paletteOutline      = color.NRGBA{R: 64, G: 72, B: 79, A: 255}
	paletteGhostBorder  = color.NRGBA{R: 64, G: 72, B: 79, A: 77}
	paletteDialogBorder = color.NRGBA{R: 168, G: 178, B: 188, A: 200}
	palettePrimaryGhost = color.NRGBA{R: 143, G: 205, B: 255, A: 77}
	palettePrimary      = color.NRGBA{R: 143, G: 205, B: 255, A: 255}
	paletteError        = color.NRGBA{R: 255, G: 180, B: 171, A: 255}
	paletteSurfLowest   = color.NRGBA{R: 10, G: 15, B: 20, A: 255}
	paletteSurfLow      = color.NRGBA{R: 23, G: 28, B: 34, A: 255}
)

var (
	BorderSubtle  color.Color = paletteGhostBorder
	BorderDefault color.Color = paletteOutline
	BorderStrong  color.Color = paletteDialogBorder
	BorderAccent  color.Color = palettePrimaryGhost
)

var (
	ActionPrimary color.Color = palettePrimary
	ActionDanger  color.Color = paletteError
)

var (
	SurfacePanel  color.Color = paletteSurfLowest
	SurfaceRaised color.Color = paletteSurfLow
)
