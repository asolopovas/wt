package decor

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
)

func NewSectionHeader(label string) fyne.CanvasObject {
	t := canvas.NewText(label, TextMuted)
	t.TextSize = TextCaption
	t.TextStyle = fyne.TextStyle{Monospace: true, Bold: true}
	return container.NewVBox(t, VGap(SpaceXS))
}

func NewSectionDivider() fyne.CanvasObject {
	line := canvas.NewRectangle(BorderSubtle)
	line.SetMinSize(fyne.NewSize(0, 1))
	pad := VGap(SpaceMD)
	return container.NewVBox(pad, line, pad)
}

func NewFormField(label string, w fyne.CanvasObject) *fyne.Container {
	lbl := canvas.NewText(label, TextMuted)
	lbl.TextSize = TextCaption
	lbl.TextStyle = fyne.TextStyle{Bold: true}
	return container.NewVBox(lbl, w)
}

func NewCaptionText(label string) *canvas.Text {
	t := canvas.NewText(label, TextMuted)
	t.TextSize = TextCaption
	t.TextStyle = fyne.TextStyle{Bold: true}
	return t
}

func NewPanelBackground() *canvas.Rectangle {
	bg := canvas.NewRectangle(SurfacePanel)
	bg.StrokeColor = BorderSubtle
	bg.StrokeWidth = 1
	return bg
}
