package gui

import "fyne.io/fyne/v2"

func notify(title, body string) {
	app := fyne.CurrentApp()
	if app == nil {
		return
	}
	app.SendNotification(&fyne.Notification{Title: title, Content: body})
}
