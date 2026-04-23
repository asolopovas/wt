package gui

import (
	"fyne.io/fyne/v2"
)

const (
	collapseWidth  = 600
	minWindowWidth = 320
	panelHeight    = 240
)

type responsiveColumns struct {
	gap float32
}

func newResponsiveColumns(gap float32) *responsiveColumns {
	return &responsiveColumns{gap: gap}
}

func (r *responsiveColumns) MinSize(objects []fyne.CanvasObject) fyne.Size {
	if len(objects) < 2 {
		return fyne.NewSize(0, 0)
	}
	return fyne.NewSize(minWindowWidth, panelHeight)
}

func (r *responsiveColumns) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	if len(objects) < 2 {
		return
	}

	if size.Width < collapseWidth {
		h := (size.Height - r.gap) / 2
		objects[0].Move(fyne.NewPos(0, 0))
		objects[0].Resize(fyne.NewSize(size.Width, h))
		objects[1].Move(fyne.NewPos(0, h+r.gap))
		objects[1].Resize(fyne.NewSize(size.Width, h))
	} else {
		colW := (size.Width - r.gap) / 2
		objects[0].Move(fyne.NewPos(0, 0))
		objects[0].Resize(fyne.NewSize(colW, size.Height))
		objects[1].Move(fyne.NewPos(colW+r.gap, 0))
		objects[1].Resize(fyne.NewSize(colW, size.Height))
	}
}

type flowLayout struct {
	gap    float32
	lastW  float32
	lastH  float32
	parent *fyne.Container
}

func newFlowLayout(gap float32) *flowLayout {
	return &flowLayout{gap: gap}
}

func (f *flowLayout) setParent(c *fyne.Container) {
	f.parent = c
}

func (f *flowLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	if len(objects) == 0 {
		return fyne.NewSize(0, 0)
	}

	maxItemW := float32(0)
	rowH := float32(0)
	for _, o := range objects {
		if !o.Visible() {
			continue
		}
		min := o.MinSize()
		if min.Width > maxItemW {
			maxItemW = min.Width
		}
		if min.Height > rowH {
			rowH = min.Height
		}
	}

	h := rowH
	if f.lastW > 0 {
		h = f.lastH
	}
	if h < rowH {
		h = rowH
	}

	return fyne.NewSize(maxItemW, h)
}

func (f *flowLayout) Layout(objects []fyne.CanvasObject, containerSize fyne.Size) {
	maxW := containerSize.Width

	type item struct {
		obj fyne.CanvasObject
		min fyne.Size
	}
	type row struct {
		items  []item
		width  float32
		height float32
	}

	var rows []row
	cur := row{}
	for _, o := range objects {
		if !o.Visible() {
			continue
		}
		min := o.MinSize()
		needed := min.Width
		if len(cur.items) > 0 {
			needed += f.gap
		}
		if cur.width+needed > maxW && len(cur.items) > 0 {
			rows = append(rows, cur)
			cur = row{}
		}
		if len(cur.items) > 0 {
			cur.width += f.gap
		}
		cur.width += min.Width
		if min.Height > cur.height {
			cur.height = min.Height
		}
		cur.items = append(cur.items, item{obj: o, min: min})
	}
	if len(cur.items) > 0 {
		rows = append(rows, cur)
	}

	y := float32(0)
	for _, r := range rows {
		x := float32(0)
		for _, it := range r.items {
			it.obj.Resize(it.min)
			it.obj.Move(fyne.NewPos(x, y))
			x += it.min.Width + f.gap
		}
		y += r.height + f.gap
	}

	newH := y - f.gap
	if len(rows) == 0 {
		newH = 0
	}
	if containerSize.Width != f.lastW || newH != f.lastH {
		f.lastW = containerSize.Width
		f.lastH = newH
		if f.parent != nil {
			f.parent.Refresh()
		}
	}
}
