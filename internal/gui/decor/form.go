package decor

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
)

const panelHeaderMinHeight = SpaceXXL * 2

type panelHeaderLayout struct{}

func (panelHeaderLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	var w float32
	for _, o := range objects {
		w += o.MinSize().Width
	}
	w += SpaceLG * 2
	return fyne.NewSize(w, panelHeaderMinHeight)
}

func (panelHeaderLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	if len(objects) == 0 {
		return
	}
	leftX := float32(SpaceLG)
	rightX := size.Width - SpaceLG
	for i := len(objects) - 1; i > 0; i-- {
		o := objects[i]
		m := o.MinSize()
		o.Resize(fyne.NewSize(m.Width, m.Height))
		o.Move(fyne.NewPos(rightX-m.Width, (size.Height-m.Height)/2))
		rightX -= m.Width + SpaceMD
	}
	left := objects[0]
	lm := left.MinSize()
	left.Resize(fyne.NewSize(lm.Width, lm.Height))
	left.Move(fyne.NewPos(leftX, (size.Height-lm.Height)/2))
}

func NewPanelHeader(left fyne.CanvasObject, right ...fyne.CanvasObject) fyne.CanvasObject {
	bg := canvas.NewRectangle(SurfaceRaised)
	bg.SetMinSize(fyne.NewSize(0, panelHeaderMinHeight))

	objs := []fyne.CanvasObject{left}
	for _, obj := range right {
		if obj == nil {
			continue
		}
		objs = append(objs, obj)
	}
	return container.NewStack(bg, container.New(panelHeaderLayout{}, objs...))
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
