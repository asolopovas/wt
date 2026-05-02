package decor

import (
	"image/color"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
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
	lbl := widget.NewLabel(msg)
	lbl.Wrapping = fyne.TextWrapWord
	ShowDialog(DialogConfig{
		Parent:      win,
		Title:       title,
		Body:        container.NewPadded(lbl),
		WidthFrac:   0.85,
		TopInset:    0,
		BottomInset: 0,
		Actions: []DialogAction{{
			Label: "OK",
			Kind:  KindPrimary,
		}},
	})
}

func ShowError(win fyne.Window, err error) {
	if win == nil || err == nil {
		return
	}
	lbl := widget.NewLabel(err.Error())
	lbl.Wrapping = fyne.TextWrapWord
	ShowDialog(DialogConfig{
		Parent:    win,
		Title:     "Error",
		Body:      container.NewPadded(lbl),
		WidthFrac: 0.85,
		Actions: []DialogAction{{
			Label: "OK",
			Kind:  KindPrimary,
		}},
	})
}

func ShowConfirm(win fyne.Window, title, body string, onConfirm func()) {
	if win == nil {
		return
	}
	lbl := widget.NewLabel(body)
	lbl.Wrapping = fyne.TextWrapWord
	ShowDialog(DialogConfig{
		Parent:    win,
		Title:     title,
		Body:      container.NewPadded(lbl),
		WidthFrac: 0.85,
		Actions: []DialogAction{
			{Label: "Cancel", Kind: KindSecondary},
			{Label: "OK", Kind: KindPrimary, OnTap: func() {
				if onConfirm != nil {
					onConfirm()
				}
			}},
		},
	})
}
