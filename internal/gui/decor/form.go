package decor

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
)

const panelHeaderMinHeight = SpaceXXL * 2

func NewPanelHeader(left fyne.CanvasObject, right ...fyne.CanvasObject) fyne.CanvasObject {
	bg := canvas.NewRectangle(SurfaceRaised)
	bg.SetMinSize(fyne.NewSize(0, panelHeaderMinHeight))

	row := []fyne.CanvasObject{left, layout.NewSpacer()}
	for _, obj := range right {
		if obj == nil {
			continue
		}
		row = append(row, obj)
	}
	return container.NewStack(bg, container.NewPadded(container.NewHBox(row...)))
}

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
