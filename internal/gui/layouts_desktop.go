//go:build !android

package gui

import (
	"fyne.io/fyne/v2"
)

const (
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

type cappedGrid struct {
	cols int
	gap  float32
	maxH float32
}

func newCappedGrid(cols int, gap, maxH float32) *cappedGrid {
	return &cappedGrid{cols: cols, gap: gap, maxH: maxH}
}

func (c *cappedGrid) rowHeight(objects []fyne.CanvasObject) float32 {
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
	return h
}

func (c *cappedGrid) rowCount(objects []fyne.CanvasObject) int {
	cols := c.cols
	if cols <= 0 || cols > len(objects) {
		cols = len(objects)
	}
	if cols <= 0 {
		return 0
	}
	return (len(objects) + cols - 1) / cols
}

func (c *cappedGrid) MinSize(objects []fyne.CanvasObject) fyne.Size {
	rows := c.rowCount(objects)
	if rows == 0 {
		return fyne.NewSize(0, 0)
	}
	rowH := c.rowHeight(objects)
	total := rowH*float32(rows) + c.gap*float32(rows-1)
	return fyne.NewSize(0, total)
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
	rowH := c.rowHeight(objects)
	if rowH <= 0 {
		rowH = size.Height
	}
	for i, o := range objects {
		col := i % cols
		row := i / cols
		x := float32(col) * (cellW + c.gap)
		y := float32(row) * (rowH + c.gap)
		o.Move(fyne.NewPos(x, y))
		o.Resize(fyne.NewSize(cellW, rowH))
	}
}
