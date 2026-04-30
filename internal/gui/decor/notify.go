package decor

import (
	"errors"
	"image/color"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/dialog"
)

type NotifyLevel int

const (
	NotifyInfo NotifyLevel = iota
	NotifyActive
	NotifySuccess
	NotifyWarning
	NotifyError
)

func SetStatusText(t *canvas.Text, level NotifyLevel, msg string) {
	if t == nil {
		return
	}
	upper := strings.ToUpper(msg)
	col, bold := statusStyle(level)
	t.Text = upper
	t.Color = col
	t.TextStyle = fyne.TextStyle{Monospace: true, Bold: bold}
	t.Refresh()
}

func SetStatusStyle(t *canvas.Text, level NotifyLevel) {
	if t == nil {
		return
	}
	col, bold := statusStyle(level)
	t.Color = col
	t.TextStyle = fyne.TextStyle{Monospace: true, Bold: bold}
}

func statusStyle(level NotifyLevel) (color.Color, bool) {
	switch level {
	case NotifyError:
		return StatusError, true
	case NotifyActive, NotifyWarning:
		return StatusActive, true
	case NotifySuccess:
		return StatusSuccess, true
	}
	return TextMuted, false
}

func ShowNotice(win fyne.Window, level NotifyLevel, title, msg string) {
	if win == nil {
		return
	}
	if level == NotifyError {
		dialog.ShowError(errors.New(msg), win)
		return
	}
	dialog.ShowInformation(title, msg, win)
}

func ShowError(win fyne.Window, err error) {
	if win == nil || err == nil {
		return
	}
	dialog.ShowError(err, win)
}

func ShowConfirm(win fyne.Window, title, body string, onConfirm func()) {
	if win == nil {
		return
	}
	dialog.ShowConfirm(title, body, func(ok bool) {
		if ok && onConfirm != nil {
			onConfirm()
		}
	}, win)
}
