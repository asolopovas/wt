//go:build android

package gui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

func (p *transcribePanel) openLibrary() {
	var hidePopup func()

	addBtn := newPointerButton("ADD FILES", func() {
		if hidePopup != nil {
			hidePopup()
		}
		p.onBrowse()
	})
	addBtn.Importance = widget.LowImportance

	closeBtn := newPointerButton("CLOSE", func() {
		if hidePopup != nil {
			hidePopup()
		}
	})
	closeBtn.Importance = widget.LowImportance

	buttons := container.NewGridWithColumns(2,
		borderedBtn(closeBtn, colOutline),
		borderedBtn(addBtn, colOutline),
	)
	bottomGap := canvas.NewRectangle(transparent)
	bottomGap.SetMinSize(fyne.NewSize(0, previewBottomInset()))
	actionRow := container.NewVBox(buttons, bottomGap)

	topGap := canvas.NewRectangle(transparent)
	topGap.SetMinSize(fyne.NewSize(0, previewTopInset()))

	body := container.NewBorder(topGap, actionRow, nil, nil, p.history.container)

	pop := widget.NewModalPopUp(dialogBordered(body), p.window.Canvas())
	pop.Resize(libraryDialogSize(p.window))
	hidePopup = pop.Hide
	pop.Show()
}
