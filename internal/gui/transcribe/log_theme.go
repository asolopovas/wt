package transcribe

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"

	"github.com/asolopovas/wt/internal/gui/decor"
)

type logEntryTheme struct{}

func (t *logEntryTheme) base() fyne.Theme {
	if app := fyne.CurrentApp(); app != nil {
		if s := app.Settings(); s != nil {
			if th := s.Theme(); th != nil {
				return th
			}
		}
	}
	return theme.DefaultTheme()
}

func (t *logEntryTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	if name == theme.ColorNameInputBackground {
		return decor.SurfacePanel
	}
	return t.base().Color(name, variant)
}

func (t *logEntryTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return t.base().Icon(name)
}

func (t *logEntryTheme) Font(style fyne.TextStyle) fyne.Resource {
	return t.base().Font(style)
}

func (t *logEntryTheme) Size(name fyne.ThemeSizeName) float32 {
	return t.base().Size(name)
}
