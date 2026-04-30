package gui

import (
	"fmt"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

func showTimePicker(parent fyne.Window, current time.Time, onSelect func(h, m, s int)) {
	hours := makeRange(0, 23, "%02d")
	minutes := makeRange(0, 59, "%02d")

	hourSel := widget.NewSelect(hours, nil)
	minSel := widget.NewSelect(minutes, nil)

	hourSel.SetSelected(fmt.Sprintf("%02d", current.Hour()))
	minSel.SetSelected(fmt.Sprintf("%02d", current.Minute()))

	colon := widget.NewLabel(":")
	row := container.NewCenter(container.NewHBox(hourSel, colon, minSel))

	showDialog(dialogConfig{
		Parent: parent,
		Title:  "PICK TIME",
		Body:   row,
		Actions: []dialogAction{
			{Label: "CANCEL", Kind: kindSecondary},
			{Label: "NOW", Kind: kindSecondary, OnTap: func() {
				now := time.Now()
				onSelect(now.Hour(), now.Minute(), now.Second())
			}},
			{Label: "OK", Kind: kindPrimary, OnTap: func() {
				var h, m int
				_, _ = fmt.Sscanf(hourSel.Selected, "%d", &h)
				_, _ = fmt.Sscanf(minSel.Selected, "%d", &m)
				onSelect(h, m, 0)
			}},
		},
		WidthFrac: 0.6,
	})
}

func makeRange(lo, hi int, format string) []string {
	out := make([]string, 0, hi-lo+1)
	for i := lo; i <= hi; i++ {
		out = append(out, fmt.Sprintf(format, i))
	}
	return out
}
