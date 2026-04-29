//go:build !android

package gui

import (
	"fmt"
	"net/url"
	"os"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
)

func (p *transcribePanel) onOpen() {
	if p.lastCSVPath == "" {
		dialog.ShowInformation("Open", "No output yet. Transcribe a file first.", p.window)
		return
	}

	if _, err := os.Stat(p.lastCSVPath); err != nil {
		dialog.ShowError(fmt.Errorf("output file not found: %s", p.lastCSVPath), p.window)
		return
	}

	u, err := url.Parse(storage.NewFileURI(p.lastCSVPath).String())
	if err != nil {
		p.appendLog(fmt.Sprintf("Open failed: %v", err))
		return
	}
	if err := fyne.CurrentApp().OpenURL(u); err != nil {
		p.appendLog(fmt.Sprintf("Open failed: %v", err))
	}
}

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
