//go:build !android

package gui

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
)

func (p *transcribePanel) onBrowse() {
	fd := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
		if err != nil || reader == nil {
			return
		}
		defer func() {
			_ = reader.Close()
		}()
		path := reader.URI().Path()
		if !p.hasFile(path) {
			p.files = append(p.files, path)
			p.rebuildChips()
			p.updateDropLabel()
		}
	}, p.window)

	fd.SetFilter(storage.NewExtensionFileFilter(audioExtensionList))
	fd.Show()
}

func (p *transcribePanel) updateDropLabel() {
	if len(p.files) > 0 {
		p.dropText.Text = fmt.Sprintf("%d file(s) added — drop more or click to browse", len(p.files))
	} else {
		p.dropText.Text = "Drop audio files here or click to browse"
	}
	p.dropText.Refresh()
}
