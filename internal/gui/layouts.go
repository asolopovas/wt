package gui

import (
	"fyne.io/fyne/v2"
)

type tightVBox struct {
	gap float32
}

func (t *tightVBox) MinSize(objects []fyne.CanvasObject) fyne.Size {
	var w, h float32
	for i, o := range objects {
		m := o.MinSize()
		if m.Width > w {
			w = m.Width
		}
		h += m.Height
		if i > 0 {
			h += t.gap
		}
	}
	return fyne.NewSize(w, h)
}

func (t *tightVBox) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	y := float32(0)
	for _, o := range objects {
		m := o.MinSize()
		o.Move(fyne.NewPos(0, y))
		o.Resize(fyne.NewSize(size.Width, m.Height))
		y += m.Height + t.gap
	}
}
