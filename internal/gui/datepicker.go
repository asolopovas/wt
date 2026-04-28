package gui

import (
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

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
