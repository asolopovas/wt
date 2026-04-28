package gui

import (
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// showDatePicker opens a modal calendar widget for picking a date, mirroring
// the time picker UX. Used because Fyne's mobile DateEntry tap behavior
// otherwise focuses the underlying entry and pops the soft keyboard.
func showDatePicker(parent fyne.Window, current time.Time, onSelect func(time.Time)) {
	picked := current
	cal := widget.NewCalendar(current, func(t time.Time) {
		picked = t
	})
	dialog.ShowCustomConfirm("Pick date", "OK", "CANCEL", cal, func(ok bool) {
		if !ok {
			return
		}
		onSelect(picked)
	}, parent)
}
