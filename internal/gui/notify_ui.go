package gui

import (
	"errors"
	"image/color"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/dialog"
)

type notifyLevel int

const (
	notifyInfo notifyLevel = iota
	notifyActive
	notifySuccess
	notifyWarning
	notifyError
)

func setStatusText(t *canvas.Text, level notifyLevel, msg string) {
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

func setStatusStyle(t *canvas.Text, level notifyLevel) {
	if t == nil {
		return
	}
	col, bold := statusStyle(level)
	t.Color = col
	t.TextStyle = fyne.TextStyle{Monospace: true, Bold: bold}
}

func statusStyle(level notifyLevel) (color.Color, bool) {
	switch level {
	case notifyError:
		return colError, true
	case notifyActive, notifyWarning:
		return colSecondary, true
	case notifySuccess:
		return colSuccess, true
	}
	return colMuted, false
}

func showNotice(win fyne.Window, level notifyLevel, title, msg string) {
	if win == nil {
		return
	}
	if level == notifyError {
		dialog.ShowError(errors.New(msg), win)
		return
	}
	dialog.ShowInformation(title, msg, win)
}

func showError(win fyne.Window, err error) {
	if win == nil || err == nil {
		return
	}
	dialog.ShowError(err, win)
}

func showConfirm(win fyne.Window, title, body string, onConfirm func()) {
	if win == nil {
		return
	}
	dialog.ShowConfirm(title, body, func(ok bool) {
		if ok && onConfirm != nil {
			onConfirm()
		}
	}, win)
}
