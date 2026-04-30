package decor

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

type PointerButton struct {
	widget.Button
}

func NewPointerButton(label string, onTap func()) *PointerButton {
	b := &PointerButton{}
	b.Text = label
	b.OnTapped = onTap
	b.ExtendBaseWidget(b)
	return b
}

func NewPointerButtonWithIcon(label string, icon fyne.Resource, onTap func()) *PointerButton {
	b := &PointerButton{}
	b.Text = label
	b.Icon = icon
	b.OnTapped = onTap
	b.ExtendBaseWidget(b)
	return b
}

func (b *PointerButton) Cursor() desktop.Cursor {
	return desktop.PointerCursor
}

func NewPrimaryButton(label string, onTap func()) *PointerButton {
	b := NewPointerButton(label, onTap)
	b.Importance = widget.HighImportance
	return b
}

func NewSecondaryButton(label string, onTap func()) *PointerButton {
	b := NewPointerButton(label, onTap)
	b.Importance = widget.LowImportance
	return b
}

func NewDangerButton(label string, onTap func()) *PointerButton {
	b := NewPointerButton(label, onTap)
	b.Importance = widget.DangerImportance
	return b
}

func WrapAction(b *PointerButton) fyne.CanvasObject {
	return BorderedBtn(b, BorderColorFor(b))
}

func WrapGhost(b *PointerButton) fyne.CanvasObject {
	return BorderedBtn(b, BorderAccent)
}

func BorderColorFor(b *PointerButton) color.Color {
	switch b.Importance {
	case widget.HighImportance:
		return ActionPrimary
	case widget.DangerImportance:
		return ActionDanger
	}
	return BorderDefault
}
