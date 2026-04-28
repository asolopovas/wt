package gui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

// tappableEntry behaves like widget.Entry visually but opens a picker on tap
// and on focus instead of accepting keyboard input. Used so the time field
// reads the same as the date field (where tap opens calendar) — picker is
// the only way to set the value.
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
	// Skip the default focus behavior so the on-screen keyboard never opens
	// and we don't enter editing mode.
	if e.onTap != nil {
		e.onTap()
	}
}

func (e *tappableEntry) FocusLost() {}

func (e *tappableEntry) AcceptsTab() bool { return false }

func (e *tappableEntry) TypedRune(_ rune) {}
func (e *tappableEntry) TypedKey(_ *fyne.KeyEvent) {}
