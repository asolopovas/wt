package gui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/widget"
)

type truncText struct {
	widget.BaseWidget
	full string
	text *canvas.Text
}

func newTruncText(s string, c color.Color, size float32, style fyne.TextStyle) *truncText {
	txt := canvas.NewText(s, c)
	txt.TextSize = size
	txt.TextStyle = style
	t := &truncText{full: s, text: txt}
	t.ExtendBaseWidget(t)
	return t
}

func (t *truncText) SetText(s string) {
	t.full = s
	t.text.Text = s
	t.Refresh()
}

func (t *truncText) MinSize() fyne.Size {
	return fyne.NewSize(0, t.text.MinSize().Height)
}

func (t *truncText) CreateRenderer() fyne.WidgetRenderer {
	return &truncTextRenderer{w: t}
}

type truncTextRenderer struct {
	w *truncText
}

func (r *truncTextRenderer) Layout(size fyne.Size) {
	r.w.text.Move(fyne.NewPos(0, 0))
	r.w.text.Resize(fyne.NewSize(size.Width, size.Height))
	full := r.w.full
	if size.Width <= 0 {
		r.w.text.Text = full
		r.w.text.Refresh()
		return
	}
	natural := fyne.MeasureText(full, r.w.text.TextSize, r.w.text.TextStyle).Width
	if natural <= size.Width {
		r.w.text.Text = full
		r.w.text.Refresh()
		return
	}
	ellipsis := "…"
	runes := []rune(full)
	lo, hi := 0, len(runes)
	for lo < hi {
		mid := (lo + hi + 1) / 2
		cand := string(runes[:mid]) + ellipsis
		if fyne.MeasureText(cand, r.w.text.TextSize, r.w.text.TextStyle).Width <= size.Width {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	r.w.text.Text = string(runes[:lo]) + ellipsis
	r.w.text.Refresh()
}

func (r *truncTextRenderer) MinSize() fyne.Size {
	return fyne.NewSize(0, r.w.text.MinSize().Height)
}

func (r *truncTextRenderer) Refresh() {
	r.w.text.Refresh()
}

func (r *truncTextRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.w.text}
}

func (r *truncTextRenderer) Destroy() {}
