//go:build !android

package gui

import (
	"fmt"
	"net/url"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/storage"
)

func openExternal(p *transcribePanel, path string) {
	u, err := url.Parse(storage.NewFileURI(path).String())
	if err != nil {
		p.appendLog(fmt.Sprintf("Open failed: %v", err))
		return
	}
	if err := fyne.CurrentApp().OpenURL(u); err != nil {
		p.appendLog(fmt.Sprintf("Open failed: %v", err))
	}
}
