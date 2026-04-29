package gui

import (
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

func showDatePicker(parent fyne.Window, current time.Time, onSelect func(time.Time)) {
	var hidePopup func()

	cal := widget.NewCalendar(current, func(t time.Time) {
		if hidePopup != nil {
			hidePopup()
		}
		onSelect(t)
	})

	nowBtn := newPointerButton("NOW", func() {
		if hidePopup != nil {
			hidePopup()
		}
		onSelect(time.Now())
	})
	nowBtn.Importance = widget.LowImportance

	cancelBtn := newPointerButton("CANCEL", func() {
		if hidePopup != nil {
			hidePopup()
		}
	})
	cancelBtn.Importance = widget.LowImportance

	buttons := container.NewGridWithColumns(2,
		borderedBtn(cancelBtn, colOutline),
		borderedBtn(nowBtn, colOutline),
	)
	bottomGap := canvas.NewRectangle(transparent)
	bottomGap.SetMinSize(fyne.NewSize(0, previewBottomInset()))
	actionRow := container.NewVBox(buttons, bottomGap)

	titleLabel := canvas.NewText("PICK DATE", colMuted)
	titleLabel.TextSize = 10
	titleLabel.TextStyle = fyne.TextStyle{Bold: true}

	topGap := canvas.NewRectangle(transparent)
	topGap.SetMinSize(fyne.NewSize(0, previewTopInset()))
	top := container.NewVBox(topGap, container.NewHBox(titleLabel))

	body := container.NewBorder(top, actionRow, nil, nil, container.NewCenter(cal))
	pop := widget.NewModalPopUp(dialogBordered(body), parent.Canvas())

	winSize := parent.Canvas().Size()
	pop.Resize(fyne.NewSize(winSize.Width*0.6, pop.MinSize().Height))
	hidePopup = pop.Hide
	pop.Show()
}
