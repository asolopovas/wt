package gui

import (
	"fyne.io/fyne/v2"
)

const (
	collapseWidth  = 600
	minWindowWidth = 320
	panelHeight    = 240

	sidebarMaxWidth   = 300
	sidebarMinWidth   = 260
	sidebarStackBelow = 820
)

type sidebarLayout struct {
	gap float32
}

func newSidebarLayout(gap float32) *sidebarLayout {
	return &sidebarLayout{gap: gap}
}

func (s *sidebarLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	if len(objects) < 2 {
		return fyne.NewSize(0, 0)
	}
	mainMin := objects[0].MinSize()
	sideMin := objects[1].MinSize()
	w := mainMin.Width
	if sideMin.Width > w {
		w = sideMin.Width
	}
	return fyne.NewSize(w, mainMin.Height)
}

func (s *sidebarLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	if len(objects) < 2 {
		return
	}
	main, side := objects[0], objects[1]

	if size.Width < sidebarStackBelow {
		sideMin := side.MinSize()
		sideH := sideMin.Height
		if sideH > size.Height*0.55 {
			sideH = size.Height * 0.55
		}
		mainH := size.Height - sideH - s.gap
		if mainH < 0 {
			mainH = 0
		}
		main.Move(fyne.NewPos(0, 0))
		main.Resize(fyne.NewSize(size.Width, mainH))
		side.Move(fyne.NewPos(0, mainH+s.gap))
		side.Resize(fyne.NewSize(size.Width, sideH))
		return
	}

	sbW := size.Width * 0.28
	if sbW > sidebarMaxWidth {
		sbW = sidebarMaxWidth
	}
	if sbW < sidebarMinWidth {
		sbW = sidebarMinWidth
	}
	mainW := size.Width - sbW - s.gap
	main.Move(fyne.NewPos(0, 0))
	main.Resize(fyne.NewSize(mainW, size.Height))
	side.Move(fyne.NewPos(mainW+s.gap, 0))
	side.Resize(fyne.NewSize(sbW, size.Height))
}

type topRightFloater struct {
	insetX float32
	insetY float32
}

func newTopRightFloater(insetX, insetY float32) *topRightFloater {
	return &topRightFloater{insetX: insetX, insetY: insetY}
}

func (f *topRightFloater) MinSize(_ []fyne.CanvasObject) fyne.Size {
	return fyne.NewSize(0, 0)
}

func (f *topRightFloater) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	for _, o := range objects {
		m := o.MinSize()
		o.Resize(m)
		o.Move(fyne.NewPos(size.Width-m.Width-f.insetX, f.insetY))
	}
}

type cappedGrid struct {
	cols int
	gap  float32
	maxH float32
}

func newCappedGrid(cols int, gap, maxH float32) *cappedGrid {
	return &cappedGrid{cols: cols, gap: gap, maxH: maxH}
}

func (c *cappedGrid) MinSize(objects []fyne.CanvasObject) fyne.Size {
	h := float32(0)
	for _, o := range objects {
		m := o.MinSize().Height
		if m > h {
			h = m
		}
	}
	if c.maxH > 0 && h > c.maxH {
		h = c.maxH
	}
	return fyne.NewSize(0, h)
}

func (c *cappedGrid) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	if len(objects) == 0 {
		return
	}
	cols := c.cols
	if cols <= 0 || cols > len(objects) {
		cols = len(objects)
	}
	cellW := (size.Width - c.gap*float32(cols-1)) / float32(cols)
	if cellW < 0 {
		cellW = 0
	}
	h := size.Height
	if c.maxH > 0 && h > c.maxH {
		h = c.maxH
	}
	for i, o := range objects {
		col := i % cols
		row := i / cols
		x := float32(col) * (cellW + c.gap)
		y := float32(row) * (h + c.gap)
		o.Move(fyne.NewPos(x, y))
		o.Resize(fyne.NewSize(cellW, h))
	}
}

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

	avail := size.Height - r.gap
	if avail < 0 {
		avail = 0
	}
	topH := avail * 0.25
	bottomH := avail - topH
	objects[0].Move(fyne.NewPos(0, 0))
	objects[0].Resize(fyne.NewSize(size.Width, topH))
	objects[1].Move(fyne.NewPos(0, topH+r.gap))
	objects[1].Resize(fyne.NewSize(size.Width, bottomH))
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
