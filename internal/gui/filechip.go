package gui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type fileChip struct {
	widget.BaseWidget
	name     string
	onClose  func()
	label    *canvas.Text
	closeBtn *pointerButton
	bg       *canvas.Rectangle
}

func newFileChip(name string, onClose func()) *fileChip {
	c := &fileChip{name: name, onClose: onClose}

	c.label = canvas.NewText(name, colPrimary)
	c.label.TextSize = 10
	c.label.TextStyle = fyne.TextStyle{Monospace: true}

	c.closeBtn = newPointerButtonWithIcon("", theme.CancelIcon(), onClose)
	c.closeBtn.Importance = widget.LowImportance

	c.bg = canvas.NewRectangle(colSurfLow)
	c.bg.StrokeColor = colPrimaryFaint
	c.bg.StrokeWidth = 1

	c.ExtendBaseWidget(c)
	return c
}

func (c *fileChip) Cursor() desktop.Cursor {
	return desktop.PointerCursor
}

func (c *fileChip) CreateRenderer() fyne.WidgetRenderer {
	closeBtnWrap := container.NewGridWrap(fyne.NewSize(28, 28), c.closeBtn)
	inner := container.NewHBox(c.label, closeBtnWrap)
	content := container.NewStack(c.bg, container.NewPadded(inner))
	return &fileChipRenderer{chip: c, content: content}
}

type fileChipRenderer struct {
	chip    *fileChip
	content fyne.CanvasObject
}

func (r *fileChipRenderer) Layout(size fyne.Size) {
	r.content.Resize(size)
}

func (r *fileChipRenderer) MinSize() fyne.Size {
	return r.content.MinSize()
}

func (r *fileChipRenderer) Refresh() {
	r.chip.label.Text = r.chip.name
	r.chip.label.Refresh()
	r.chip.closeBtn.Refresh()
	r.chip.bg.Refresh()
	r.content.Refresh()
}

func (r *fileChipRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.content}
}

func (r *fileChipRenderer) Destroy() {}
