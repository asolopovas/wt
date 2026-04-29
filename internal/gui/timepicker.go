package gui

import (
	"fmt"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
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

	var hidePopup func()

	nowBtn := newPointerButton("NOW", func() {
		if hidePopup != nil {
			hidePopup()
		}
		now := time.Now()
		onSelect(now.Hour(), now.Minute(), now.Second())
	})
	nowBtn.Importance = widget.LowImportance

	cancelBtn := newPointerButton("CANCEL", func() {
		if hidePopup != nil {
			hidePopup()
		}
	})
	cancelBtn.Importance = widget.LowImportance

	okBtn := newPointerButton("OK", func() {
		if hidePopup != nil {
			hidePopup()
		}
		var h, m int
		_, _ = fmt.Sscanf(hourSel.Selected, "%d", &h)
		_, _ = fmt.Sscanf(minSel.Selected, "%d", &m)
		onSelect(h, m, 0)
	})
	okBtn.Importance = widget.LowImportance

	buttons := container.NewGridWithColumns(3,
		borderedBtn(cancelBtn, colOutline),
		borderedBtn(nowBtn, colOutline),
		borderedBtn(okBtn, colOutline),
	)
	bottomGap := canvas.NewRectangle(transparent)
	bottomGap.SetMinSize(fyne.NewSize(0, previewBottomInset()))
	actionRow := container.NewVBox(buttons, bottomGap)

	titleLabel := canvas.NewText("PICK TIME", colMuted)
	titleLabel.TextSize = 10
	titleLabel.TextStyle = fyne.TextStyle{Bold: true}

	topGap := canvas.NewRectangle(transparent)
	topGap.SetMinSize(fyne.NewSize(0, previewTopInset()))
	top := container.NewVBox(topGap, container.NewHBox(titleLabel))

	body := container.NewBorder(top, actionRow, nil, nil, row)
	pop := widget.NewModalPopUp(dialogBordered(body), parent.Canvas())

	winSize := parent.Canvas().Size()
	pop.Resize(fyne.NewSize(winSize.Width*0.6, pop.MinSize().Height))
	hidePopup = pop.Hide
	pop.Show()
}

func makeRange(lo, hi int, format string) []string {
	out := make([]string, 0, hi-lo+1)
	for i := lo; i <= hi; i++ {
		out = append(out, fmt.Sprintf(format, i))
	}
	return out
}
