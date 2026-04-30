package decor

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

type PointerSelect struct {
	widget.Select
}

func NewPointerSelect(options []string, changed func(string)) *PointerSelect {
	s := &PointerSelect{}
	s.Options = options
	s.OnChanged = changed
	s.ExtendBaseWidget(s)
	return s
}

func (s *PointerSelect) Cursor() desktop.Cursor {
	return desktop.PointerCursor
}

type LimitSelect struct {
	widget.BaseWidget

	Inner     *widget.Select
	maxHeight float32
}

func NewLimitSelect(options []string, maxHeight float32, changed func(string)) *LimitSelect {
	s := &LimitSelect{maxHeight: maxHeight}
	s.Inner = widget.NewSelect(options, changed)
	s.ExtendBaseWidget(s)
	return s
}

func (s *LimitSelect) Tapped(_ *fyne.PointEvent) {
	if s.Inner.Disabled() {
		return
	}

	items := make([]*fyne.MenuItem, len(s.Inner.Options))
	for i := range s.Inner.Options {
		text := s.Inner.Options[i]
		items[i] = fyne.NewMenuItem(text, func() {
			s.Inner.SetSelected(text)
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

func (s *LimitSelect) Cursor() desktop.Cursor            { return desktop.PointerCursor }
func (s *LimitSelect) MouseIn(ev *desktop.MouseEvent)    { s.Inner.MouseIn(ev) }
func (s *LimitSelect) MouseMoved(ev *desktop.MouseEvent) { s.Inner.MouseMoved(ev) }
func (s *LimitSelect) MouseOut()                         { s.Inner.MouseOut() }

func (s *LimitSelect) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(s.Inner)
}
