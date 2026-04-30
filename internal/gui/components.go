package gui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

func newPrimaryButton(label string, onTap func()) *pointerButton {
	b := newPointerButton(label, onTap)
	b.Importance = widget.HighImportance
	return b
}

func newSecondaryButton(label string, onTap func()) *pointerButton {
	b := newPointerButton(label, onTap)
	b.Importance = widget.LowImportance
	return b
}

func newDangerButton(label string, onTap func()) *pointerButton {
	b := newPointerButton(label, onTap)
	b.Importance = widget.DangerImportance
	return b
}

func wrapAction(b *pointerButton) fyne.CanvasObject {
	return borderedBtn(b, borderColorFor(b))
}

func wrapGhost(b *pointerButton) fyne.CanvasObject {
	return borderedBtn(b, borderAccent)
}

func borderColorFor(b *pointerButton) color.Color {
	switch b.Importance {
	case widget.HighImportance:
		return actionPrimary
	case widget.DangerImportance:
		return actionDanger
	}
	return borderDefault
}

func newSectionHeader(label string) fyne.CanvasObject {
	t := canvas.NewText(label, colMuted)
	t.TextSize = textCaption
	t.TextStyle = fyne.TextStyle{Monospace: true, Bold: true}
	return container.NewVBox(t, vGap(spaceXS))
}

func newSectionDivider() fyne.CanvasObject {
	line := canvas.NewRectangle(borderSubtle)
	line.SetMinSize(fyne.NewSize(0, 1))
	pad := vGap(spaceMD)
	return container.NewVBox(pad, line, pad)
}

func newFormField(label string, w fyne.CanvasObject) *fyne.Container {
	lbl := canvas.NewText(label, colMuted)
	lbl.TextSize = textCaption
	lbl.TextStyle = fyne.TextStyle{Bold: true}
	return container.NewVBox(lbl, w)
}

func newCaptionText(label string) *canvas.Text {
	t := canvas.NewText(label, colMuted)
	t.TextSize = textCaption
	t.TextStyle = fyne.TextStyle{Bold: true}
	return t
}

func newPanelBackground() *canvas.Rectangle {
	bg := canvas.NewRectangle(surfacePanel)
	bg.StrokeColor = borderSubtle
	bg.StrokeWidth = 1
	return bg
}

func vGap(h float32) fyne.CanvasObject {
	r := canvas.NewRectangle(transparent)
	r.SetMinSize(fyne.NewSize(0, h))
	return r
}

func hGap(w float32) fyne.CanvasObject {
	r := canvas.NewRectangle(transparent)
	r.SetMinSize(fyne.NewSize(w, 0))
	return r
}

type dialogAction struct {
	Label string
	OnTap func()
	Kind  buttonKind
}

type buttonKind int

const (
	kindSecondary buttonKind = iota
	kindPrimary
	kindDanger
)

type dialogConfig struct {
	Parent    fyne.Window
	Title     string
	Body      fyne.CanvasObject
	Actions   []dialogAction
	WidthFrac float32
	Size      *fyne.Size
}

func showDialog(cfg dialogConfig) func() {
	if cfg.Parent == nil {
		return func() {}
	}

	var hide func()

	actionObjs := make([]fyne.CanvasObject, 0, len(cfg.Actions))
	for _, a := range cfg.Actions {
		action := a
		var btn *pointerButton
		switch action.Kind {
		case kindPrimary:
			btn = newPrimaryButton(action.Label, nil)
		case kindDanger:
			btn = newDangerButton(action.Label, nil)
		default:
			btn = newSecondaryButton(action.Label, nil)
		}
		btn.OnTapped = func() {
			if hide != nil {
				hide()
			}
			if action.OnTap != nil {
				action.OnTap()
			}
		}
		actionObjs = append(actionObjs, wrapAction(btn))
	}

	bottomGap := vGap(previewBottomInset())
	bottom := bottomGap
	if len(actionObjs) > 0 {
		row := container.NewGridWithColumns(len(actionObjs), actionObjs...)
		bottom = container.NewVBox(row, bottomGap)
	}

	topGap := vGap(previewTopInset())
	top := topGap
	if cfg.Title != "" {
		top = container.NewVBox(topGap, container.NewHBox(newSectionHeader(cfg.Title)))
	}

	bodyContainer := container.NewBorder(top, bottom, nil, nil, cfg.Body)
	pop := widget.NewModalPopUp(dialogBordered(bodyContainer), cfg.Parent.Canvas())

	if cfg.Size != nil {
		pop.Resize(*cfg.Size)
	} else if cfg.WidthFrac > 0 {
		winSize := cfg.Parent.Canvas().Size()
		pop.Resize(fyne.NewSize(winSize.Width*cfg.WidthFrac, pop.MinSize().Height))
	}

	hide = pop.Hide
	pop.Show()
	return hide
}
