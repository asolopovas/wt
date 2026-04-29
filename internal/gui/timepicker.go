package gui

import (
	"fmt"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

func showTimePicker(parent fyne.Window, current time.Time, onSelect func(h, m, s int)) {
	hours := makeRange(0, 23, "%02d")
	minutes := makeRange(0, 59, "%02d")
	seconds := makeRange(0, 59, "%02d")

	hourSel := widget.NewSelect(hours, nil)
	minSel := widget.NewSelect(minutes, nil)
	secSel := widget.NewSelect(seconds, nil)

	hourSel.SetSelected(fmt.Sprintf("%02d", current.Hour()))
	minSel.SetSelected(fmt.Sprintf("%02d", current.Minute()))
	secSel.SetSelected(fmt.Sprintf("%02d", current.Second()))

	colon1 := widget.NewLabel(":")
	colon2 := widget.NewLabel(":")
	row := container.NewHBox(hourSel, colon1, minSel, colon2, secSel)

	dialog.ShowCustomConfirm("Pick time", "OK", "CANCEL", row, func(ok bool) {
		if !ok {
			return
		}
		var h, m, s int
		_, _ = fmt.Sscanf(hourSel.Selected, "%d", &h)
		_, _ = fmt.Sscanf(minSel.Selected, "%d", &m)
		_, _ = fmt.Sscanf(secSel.Selected, "%d", &s)
		onSelect(h, m, s)
	}, parent)
}

func makeRange(lo, hi int, format string) []string {
	out := make([]string, 0, hi-lo+1)
	for i := lo; i <= hi; i++ {
		out = append(out, fmt.Sprintf(format, i))
	}
	return out
}
