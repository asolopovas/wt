//go:build !android

package transcribe

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
)

func (p *Panel) OnBrowse() {
	fd := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
		if err != nil {
			p.AppendLog(fmt.Sprintf("Error opening file: %v", err))
			return
		}
		if reader == nil {
			return
		}
		defer func() {
			_ = reader.Close()
		}()
		path := reader.URI().Path()
		if p.AddLocalFile(path) {
			p.RebuildChips()
			p.UpdateDropLabel()
		}
	}, p.window)

	fd.SetFilter(storage.NewExtensionFileFilter(audioExtensionList))
	fd.Show()
}

func (p *Panel) UpdateDropLabel() {
	if p.dropText == nil {
		return
	}
	if len(p.files) > 0 {
		p.dropText.Text = fmt.Sprintf("%d file(s) added — drop more or click to browse", len(p.files))
	} else {
		p.dropText.Text = "Drop audio files here or click to browse"
	}
	p.dropText.Refresh()
}
