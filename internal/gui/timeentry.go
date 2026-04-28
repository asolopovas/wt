package gui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

type tappableEntry struct {
	widget.Entry
	onTap func()
}

func newTappableEntry(onTap func()) *tappableEntry {
	e := &tappableEntry{onTap: onTap}
	e.ExtendBaseWidget(e)
	return e
}

func (e *tappableEntry) Tapped(_ *fyne.PointEvent) {
	if e.onTap != nil {
		e.onTap()
	}
}

func (e *tappableEntry) TappedSecondary(_ *fyne.PointEvent) {}

func (e *tappableEntry) FocusGained() {
	if e.onTap != nil {
		e.onTap()
	}
}

func (e *tappableEntry) FocusLost() {}

func (e *tappableEntry) AcceptsTab() bool { return false }

func (e *tappableEntry) TypedRune(_ rune)          {}
func (e *tappableEntry) TypedKey(_ *fyne.KeyEvent) {}
