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

	hourSel := widget.NewSelect(hours, nil)
	minSel := widget.NewSelect(minutes, nil)

	hourSel.SetSelected(fmt.Sprintf("%02d", current.Hour()))
	minSel.SetSelected(fmt.Sprintf("%02d", current.Minute()))

	colon1 := widget.NewLabel(":")
	row := container.NewHBox(hourSel, colon1, minSel)

	d := dialog.NewCustomWithoutButtons("Pick time", dialogBordered(row), parent)

	nowBtn := widget.NewButton("NOW", func() {
		d.Hide()
		now := time.Now()
		onSelect(now.Hour(), now.Minute(), now.Second())
	})
	cancelBtn := widget.NewButton("CANCEL", func() { d.Hide() })
	okBtn := widget.NewButton("OK", func() {
		d.Hide()
		var h, m int
		_, _ = fmt.Sscanf(hourSel.Selected, "%d", &h)
		_, _ = fmt.Sscanf(minSel.Selected, "%d", &m)
		onSelect(h, m, 0)
	})
	okBtn.Importance = widget.HighImportance

	d.SetButtons([]fyne.CanvasObject{nowBtn, cancelBtn, okBtn})
	d.Show()
	winSize := parent.Canvas().Size()
	d.Resize(fyne.NewSize(winSize.Width*0.8, d.MinSize().Height))
}

func makeRange(lo, hi int, format string) []string {
	out := make([]string, 0, hi-lo+1)
	for i := lo; i <= hi; i++ {
		out = append(out, fmt.Sprintf(format, i))
	}
	return out
}
