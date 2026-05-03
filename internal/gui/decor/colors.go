package decor

import "image/color"

var Transparent = color.NRGBA{}

var (
	paletteOutline      = color.NRGBA{R: 64, G: 72, B: 79, A: 255}
	paletteGhostBorder  = color.NRGBA{R: 64, G: 72, B: 79, A: 77}
	paletteDialogBorder = color.NRGBA{R: 168, G: 178, B: 188, A: 200}
	palettePrimaryGhost = color.NRGBA{R: 143, G: 205, B: 255, A: 77}
	palettePrimary      = color.NRGBA{R: 143, G: 205, B: 255, A: 255}
	paletteError        = color.NRGBA{R: 239, G: 83, B: 80, A: 255}
	paletteSurfLowest   = color.NRGBA{R: 10, G: 15, B: 20, A: 255}
	paletteSurfLow      = color.NRGBA{R: 23, G: 28, B: 34, A: 255}
	paletteSurfHigh     = color.NRGBA{R: 37, G: 42, B: 49, A: 255}
	paletteMuted        = color.NRGBA{R: 191, G: 199, B: 208, A: 255}
	paletteSecondary    = color.NRGBA{R: 255, G: 208, B: 135, A: 255}
	paletteSuccess      = color.NRGBA{R: 187, G: 250, B: 195, A: 255}
)

var (
	BorderSubtle  color.Color = paletteGhostBorder
	BorderDefault color.Color = paletteOutline
	BorderStrong  color.Color = paletteDialogBorder
	BorderAccent  color.Color = palettePrimaryGhost
)

var (
	ActionPrimary      color.Color = palettePrimary
	ActionPrimaryFaint color.Color = color.NRGBA{R: 143, G: 205, B: 255, A: 102}
	ActionDanger       color.Color = paletteError
)

var (
	SurfacePanel  color.Color = paletteSurfLowest
	SurfaceRaised color.Color = paletteSurfLow
	SurfaceHigh   color.Color = paletteSurfHigh
)

var (
	TextPrimary   color.Color = color.White
	TextMuted     color.Color = paletteMuted
	TextSecondary color.Color = paletteSecondary
	StatusSuccess color.Color = paletteSuccess
	StatusError   color.Color = paletteError
	StatusActive  color.Color = paletteSecondary
)
