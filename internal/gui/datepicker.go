package gui

import (
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

func showDatePicker(parent fyne.Window, current time.Time, onSelect func(time.Time)) {
	var hide func()

	cal := widget.NewCalendar(current, func(t time.Time) {
		if hide != nil {
			hide()
		}
		onSelect(t)
	})

	hide = showDialog(dialogConfig{
		Parent: parent,
		Title:  "PICK DATE",
		Body:   container.NewCenter(cal),
		Actions: []dialogAction{
			{Label: "CANCEL", Kind: kindSecondary},
			{Label: "NOW", Kind: kindSecondary, OnTap: func() {
				onSelect(time.Now())
			}},
		},
		WidthFrac: 0.6,
	})
}
