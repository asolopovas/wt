package gui

import (
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

func showDatePicker(parent fyne.Window, current time.Time, onSelect func(time.Time)) {
	var d *dialog.CustomDialog
	cal := widget.NewCalendar(current, func(t time.Time) {
		if d != nil {
			d.Hide()
		}
		onSelect(t)
	})

	d = dialog.NewCustomWithoutButtons("Pick date", cal, parent)

	nowBtn := widget.NewButton("NOW", func() {
		d.Hide()
		onSelect(time.Now())
	})
	cancelBtn := widget.NewButton("CANCEL", func() { d.Hide() })

	d.SetButtons([]fyne.CanvasObject{nowBtn, cancelBtn})
	d.Show()
}
