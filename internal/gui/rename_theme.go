package gui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

type renameEntryTheme struct {
	parent fyne.Theme
}

func (t *renameEntryTheme) Color(n fyne.ThemeColorName, v fyne.ThemeVariant) color.Color {
	return t.parent.Color(n, v)
}

func (t *renameEntryTheme) Font(s fyne.TextStyle) fyne.Resource     { return t.parent.Font(s) }
func (t *renameEntryTheme) Icon(n fyne.ThemeIconName) fyne.Resource { return t.parent.Icon(n) }

func (t *renameEntryTheme) Size(n fyne.ThemeSizeName) float32 {
	switch n {
	case theme.SizeNameText:
		return 18
	case theme.SizeNameInnerPadding:
		return 12
	}
	return t.parent.Size(n)
}
