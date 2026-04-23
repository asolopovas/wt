package gui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

type limitSelect struct {
	widget.BaseWidget

	inner     *widget.Select
	maxHeight float32
}

func newLimitSelect(options []string, maxHeight float32, changed func(string)) *limitSelect {
	s := &limitSelect{maxHeight: maxHeight}
	s.inner = widget.NewSelect(options, changed)
	s.ExtendBaseWidget(s)
	return s
}

func (s *limitSelect) Tapped(_ *fyne.PointEvent) {
	if s.inner.Disabled() {
		return
	}

	items := make([]*fyne.MenuItem, len(s.inner.Options))
	for i := range s.inner.Options {
		text := s.inner.Options[i]
		items[i] = fyne.NewMenuItem(text, func() {
			s.inner.SetSelected(text)
		})
	}

	c := fyne.CurrentApp().Driver().CanvasForObject(s)
	pop := widget.NewPopUpMenu(fyne.NewMenu("", items...), c)

	buttonPos := fyne.CurrentApp().Driver().AbsolutePositionForObject(s)
	pos := buttonPos.Add(fyne.NewPos(0, s.Size().Height))

	pop.ShowAtPosition(pos)

	popH := pop.MinSize().Height
	if popH > s.maxHeight {
		popH = s.maxHeight
	}
	pop.Resize(fyne.NewSize(s.Size().Width, popH))
}

func (s *limitSelect) Cursor() desktop.Cursor            { return desktop.PointerCursor }
func (s *limitSelect) MouseIn(ev *desktop.MouseEvent)    { s.inner.MouseIn(ev) }
func (s *limitSelect) MouseMoved(ev *desktop.MouseEvent) { s.inner.MouseMoved(ev) }
func (s *limitSelect) MouseOut()                         { s.inner.MouseOut() }

func (s *limitSelect) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(s.inner)
}
