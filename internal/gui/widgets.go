package gui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

type pointerButton struct {
	widget.Button
}

func newPointerButton(label string, onTap func()) *pointerButton {
	b := &pointerButton{}
	b.Text = label
	b.OnTapped = onTap
	b.ExtendBaseWidget(b)
	return b
}

func newPointerButtonWithIcon(label string, icon fyne.Resource, onTap func()) *pointerButton {
	b := &pointerButton{}
	b.Text = label
	b.Icon = icon
	b.OnTapped = onTap
	b.ExtendBaseWidget(b)
	return b
}

func (b *pointerButton) Cursor() desktop.Cursor {
	return desktop.PointerCursor
}

type pointerSelect struct {
	widget.Select
}

func newPointerSelect(options []string, changed func(string)) *pointerSelect {
	s := &pointerSelect{}
	s.Options = options
	s.OnChanged = changed
	s.ExtendBaseWidget(s)
	return s
}

func (s *pointerSelect) Cursor() desktop.Cursor {
	return desktop.PointerCursor
}

func dialogBordered(content fyne.CanvasObject) fyne.CanvasObject {
	frame := canvas.NewRectangle(transparent)
	frame.StrokeColor = borderStrong
	frame.StrokeWidth = 1

	inner := container.NewBorder(
		vGap(spaceXL), vGap(spaceXL),
		hGap(spaceXL), hGap(spaceXL),
		content,
	)
	return container.NewStack(frame, inner)
}

func borderedBtn(btn fyne.CanvasObject, borderCol color.Color) fyne.CanvasObject {
	frame := canvas.NewRectangle(transparent)
	frame.StrokeColor = borderCol
	frame.StrokeWidth = 1
	return container.NewStack(frame, btn)
}

type thinProgress struct {
	widget.BaseWidget
	value   float64
	track   *canvas.Rectangle
	fill    *canvas.Rectangle
	visible bool
}

func newThinProgress() *thinProgress {
	p := &thinProgress{visible: false}
	p.track = canvas.NewRectangle(colSurfHigh)
	p.fill = canvas.NewRectangle(colPrimary)
	p.ExtendBaseWidget(p)
	return p
}

func (p *thinProgress) SetValue(v float64) {
	if v < 0 {
		v = 0
	}
	if v > 1 {
		v = 1
	}
	p.value = v
	p.Refresh()
}

func (p *thinProgress) Show() {
	p.visible = true
	p.BaseWidget.Show()
}

func (p *thinProgress) Hide() {
	p.visible = false
	p.BaseWidget.Hide()
}

func (p *thinProgress) CreateRenderer() fyne.WidgetRenderer {
	return &thinProgressRenderer{p: p}
}

type thinProgressRenderer struct {
	p *thinProgress
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
	r.p.track.FillColor = colSurfHigh
	r.p.fill.FillColor = colPrimary
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
