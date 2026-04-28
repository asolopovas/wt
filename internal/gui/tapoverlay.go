package gui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/widget"
)

type tapOverlay struct {
	widget.BaseWidget
	onTap func()
}

func newTapOverlay(onTap func()) *tapOverlay {
	o := &tapOverlay{onTap: onTap}
	o.ExtendBaseWidget(o)
	return o
}

func (o *tapOverlay) Tapped(_ *fyne.PointEvent) {
	if o.onTap != nil {
		o.onTap()
	}
}

func (o *tapOverlay) TappedSecondary(_ *fyne.PointEvent) {}

func (o *tapOverlay) CreateRenderer() fyne.WidgetRenderer {
	r := canvas.NewRectangle(color.Transparent)
	return widget.NewSimpleRenderer(r)
}
