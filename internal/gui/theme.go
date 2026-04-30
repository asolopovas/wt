package gui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

var (
	colBackground   = color.NRGBA{R: 15, G: 20, B: 26, A: 255}
	colSurfLowest   = color.NRGBA{R: 10, G: 15, B: 20, A: 255}
	colSurfLow      = color.NRGBA{R: 23, G: 28, B: 34, A: 255}
	colSurfMid      = color.NRGBA{R: 27, G: 32, B: 38, A: 255}
	colSurfHigh     = color.NRGBA{R: 37, G: 42, B: 49, A: 255}
	colSurfBright   = color.NRGBA{R: 53, G: 58, B: 64, A: 255}
	colPrimary      = color.NRGBA{R: 143, G: 205, B: 255, A: 255}
	colOnPrimary    = color.NRGBA{R: 0, G: 52, B: 79, A: 255}
	colForeground   = color.NRGBA{R: 222, G: 227, B: 235, A: 255}
	colMuted        = color.NRGBA{R: 191, G: 199, B: 208, A: 255}
	colSecondary    = color.NRGBA{R: 255, G: 208, B: 135, A: 255}
	colOutline      = color.NRGBA{R: 64, G: 72, B: 79, A: 255}
	colGhostBorder  = color.NRGBA{R: 64, G: 72, B: 79, A: 77}
	colDialogBorder = color.NRGBA{R: 168, G: 178, B: 188, A: 200}
	colPrimaryGhost = color.NRGBA{R: 143, G: 205, B: 255, A: 77}
	colPrimaryFaint = color.NRGBA{R: 143, G: 205, B: 255, A: 102}
	colHover        = color.NRGBA{R: 143, G: 205, B: 255, A: 20}
	colSuccess      = color.NRGBA{R: 187, G: 250, B: 195, A: 255}
	colError        = color.NRGBA{R: 255, G: 180, B: 171, A: 255}
	transparent     = color.NRGBA{}
)

type whisperTheme struct{}

func (t *whisperTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNameBackground:
		return colBackground
	case theme.ColorNameOverlayBackground, theme.ColorNameMenuBackground:
		return colSurfMid
	case theme.ColorNameButton:
		return colSurfHigh
	case theme.ColorNamePrimary:
		return colPrimary
	case theme.ColorNameForeground:
		return colForeground
	case theme.ColorNameForegroundOnPrimary:
		return colOnPrimary
	case theme.ColorNameInputBackground:
		return colSurfHigh
	case theme.ColorNameSeparator:
		return colDialogBorder
	case theme.ColorNameDisabled:
		return colSurfBright
	case theme.ColorNameDisabledButton:
		return colSurfMid
	case theme.ColorNameSuccess:
		return colSuccess
	case theme.ColorNameError:
		return colError
	case theme.ColorNameHover:
		return colHover
	case theme.ColorNameForegroundOnSuccess:
		return colOnPrimary
	case theme.ColorNameForegroundOnError:
		return colBackground
	case theme.ColorNamePlaceHolder:
		return colMuted
	case theme.ColorNameHeaderBackground:
		return colSurfLow
	case theme.ColorNameScrollBar:
		return colSurfBright
	case theme.ColorNameShadow:
		return transparent
	}
	return theme.DefaultTheme().Color(name, theme.VariantDark)
}

func (t *whisperTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}

func (t *whisperTheme) Font(style fyne.TextStyle) fyne.Resource {
	return theme.DefaultTheme().Font(style)
}

type logEntryTheme struct{ whisperTheme }

func (t *logEntryTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	if name == theme.ColorNameInputBackground || name == theme.ColorNameDisabled {
		return colSurfLowest
	}
	return t.whisperTheme.Color(name, variant)
}

func (t *whisperTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNamePadding:
		return spaceMD
	case theme.SizeNameInnerPadding:
		return spaceXL
	case theme.SizeNameText:
		return textLabel
	case theme.SizeNameHeadingText:
		return textHeading
	case theme.SizeNameSubHeadingText:
		return textRow
	case theme.SizeNameLineSpacing:
		return 0
	case theme.SizeNameScrollBarSmall:
		return spaceSM
	case theme.SizeNameScrollBar:
		return textCaption
	case theme.SizeNameInputRadius:
		return 0
	case theme.SizeNameSelectionRadius:
		return 0
	case theme.SizeNameScrollBarRadius:
		return 0
	}
	return theme.DefaultTheme().Size(name)
}
