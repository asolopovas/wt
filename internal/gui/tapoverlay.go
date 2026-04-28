package gui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/widget"
)

// tapOverlay is a fully transparent widget that captures taps and forwards
// them to onTap. Used to put a click-shield over an Entry/DateEntry so the
// underlying widget never receives focus and the soft keyboard never opens.
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
