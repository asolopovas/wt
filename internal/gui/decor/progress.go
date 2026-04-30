package decor

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/widget"
)

type ThinProgress struct {
	widget.BaseWidget
	value   float64
	track   *canvas.Rectangle
	fill    *canvas.Rectangle
	visible bool
}

func NewThinProgress() *ThinProgress {
	p := &ThinProgress{visible: false}
	p.track = canvas.NewRectangle(SurfaceHigh)
	p.fill = canvas.NewRectangle(ActionPrimary)
	p.ExtendBaseWidget(p)
	return p
}

func (p *ThinProgress) SetValue(v float64) {
	if v < 0 {
		v = 0
	}
	if v > 1 {
		v = 1
	}
	p.value = v
	p.Refresh()
}

func (p *ThinProgress) Show() {
	p.visible = true
	p.BaseWidget.Show()
}

func (p *ThinProgress) Hide() {
	p.visible = false
	p.BaseWidget.Hide()
}

func (p *ThinProgress) CreateRenderer() fyne.WidgetRenderer {
	return &thinProgressRenderer{p: p}
}

type thinProgressRenderer struct {
	p *ThinProgress
}

func (r *thinProgressRenderer) Layout(size fyne.Size) {
	r.p.track.Resize(size)
	r.p.track.Move(fyne.NewPos(0, 0))
	fillW := size.Width * float32(r.p.value)
	r.p.fill.Resize(fyne.NewSize(fillW, size.Height))
	r.p.fill.Move(fyne.NewPos(0, 0))
}

func (r *thinProgressRenderer) MinSize() fyne.Size {
	return fyne.NewSize(0, 6)
}

func (r *thinProgressRenderer) Refresh() {
	r.p.track.FillColor = SurfaceHigh
	r.p.fill.FillColor = ActionPrimary
	size := r.p.track.Size()
	fillW := size.Width * float32(r.p.value)
	r.p.fill.Resize(fyne.NewSize(fillW, size.Height))
	r.p.track.Refresh()
	r.p.fill.Refresh()
}

func (r *thinProgressRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.p.track, r.p.fill}
}

func (r *thinProgressRenderer) Destroy() {}
