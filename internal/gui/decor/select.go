package decor

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

const LimitSelectRowHeight float32 = 22

type compactPopupTheme struct {
	parent fyne.Theme
}

func (t compactPopupTheme) Color(n fyne.ThemeColorName, v fyne.ThemeVariant) color.Color {
	return t.parent.Color(n, v)
}
func (t compactPopupTheme) Font(s fyne.TextStyle) fyne.Resource     { return t.parent.Font(s) }
func (t compactPopupTheme) Icon(n fyne.ThemeIconName) fyne.Resource { return t.parent.Icon(n) }
func (t compactPopupTheme) Size(n fyne.ThemeSizeName) float32 {
	switch n {
	case theme.SizeNamePadding, theme.SizeNameInnerPadding:
		return 2
	case theme.SizeNameSeparatorThickness:
		return 0
	}
	return t.parent.Size(n)
}

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

	options := s.Inner.Options
	var pop *widget.PopUp
	list := widget.NewList(
		func() int { return len(options) },
		func() fyne.CanvasObject {
			lbl := widget.NewLabel("")
			lbl.Truncation = fyne.TextTruncateEllipsis
			return lbl
		},
		func(id widget.ListItemID, o fyne.CanvasObject) {
			if id < 0 || id >= len(options) {
				return
			}
			o.(*widget.Label).SetText(options[id])
		},
	)
	list.HideSeparators = true
	for i := range options {
		list.SetItemHeight(i, LimitSelectRowHeight)
	}
	list.OnSelected = func(id widget.ListItemID) {
		if id < 0 || id >= len(options) {
			return
		}
		s.Inner.SetSelected(options[id])
		if pop != nil {
			pop.Hide()
		}
	}

	themed := container.NewThemeOverride(list, compactPopupTheme{parent: fyne.CurrentApp().Settings().Theme()})

	c := fyne.CurrentApp().Driver().CanvasForObject(s)
	pop = widget.NewPopUp(themed, c)

	buttonPos := fyne.CurrentApp().Driver().AbsolutePositionForObject(s)
	pos := buttonPos.Add(fyne.NewPos(0, s.Size().Height))

	desired := LimitSelectRowHeight*float32(len(options)) + 8
	if desired > s.maxHeight {
		desired = s.maxHeight
	}
	pop.ShowAtPosition(pos)
	pop.Resize(fyne.NewSize(s.Size().Width, desired))
}

func (s *LimitSelect) Cursor() desktop.Cursor            { return desktop.PointerCursor }
func (s *LimitSelect) MouseIn(ev *desktop.MouseEvent)    { s.Inner.MouseIn(ev) }
func (s *LimitSelect) MouseMoved(ev *desktop.MouseEvent) { s.Inner.MouseMoved(ev) }
func (s *LimitSelect) MouseOut()                         { s.Inner.MouseOut() }

func (s *LimitSelect) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(s.Inner)
}
