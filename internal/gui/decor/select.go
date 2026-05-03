package decor

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// LimitSelectRowHeight is the per-row content height applied via
// widget.List.SetItemHeight. The actual row stride is this value plus
// theme.SizeNamePadding (compactPopupTheme reduces the padding too,
// so the resulting row is ~30 px tall on Android instead of ~80 px).
const LimitSelectRowHeight float32 = 22

// compactPopupTheme delegates everything to the parent theme except
// SizeNamePadding / SizeNameInnerPadding which are clamped low. Used
// inside container.NewThemeOverride wrapping a list popup so dropdown
// rows aren't padded out to 80 px on high-DPI Android.
type compactPopupTheme struct {
	parent fyne.Theme
}

func (t compactPopupTheme) Color(n fyne.ThemeColorName, v fyne.ThemeVariant) color.Color {
	return t.parent.Color(n, v)
}
func (t compactPopupTheme) Font(s fyne.TextStyle) fyne.Resource { return t.parent.Font(s) }
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

	// Use widget.List instead of widget.PopUpMenu so we control per-row
	// height. PopUpMenu's row height is theme-padding-driven (~80 px on
	// Android high-DPI) which makes long lists like the 99-language
	// picker waste screen space and require excessive scrolling.
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
		list.SetItemHeight(widget.ListItemID(i), LimitSelectRowHeight)
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

	// Wrap in ThemeOverride so theme.SizeNamePadding shrinks just for
	// this popup. Without this the list adds ~30 px between every row
	// on high-DPI Android (see widget/list.go:412 paddedItemHeight =
	// itemHeight + padding).
	themed := container.NewThemeOverride(list, compactPopupTheme{parent: fyne.CurrentApp().Settings().Theme()})

	c := fyne.CurrentApp().Driver().CanvasForObject(s)
	pop = widget.NewPopUp(themed, c)

	buttonPos := fyne.CurrentApp().Driver().AbsolutePositionForObject(s)
	pos := buttonPos.Add(fyne.NewPos(0, s.Size().Height))

	// Sized: row height × visible rows, capped at maxHeight. This keeps
	// short lists (e.g. SenseVoice's 6 entries) from showing empty space.
	desired := LimitSelectRowHeight*float32(len(options)) + 8 // +scrollbar+padding
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
