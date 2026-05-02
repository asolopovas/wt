//go:build !android

package transcribe

import (
	"fmt"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"

	"github.com/asolopovas/wt/internal/transcriber"
)

func (p *Panel) exportBundleFolder(tr *transcriber.Transcript, item ExportItem, start time.Time) {
	folderDialog := dialog.NewFolderOpen(func(u fyne.ListableURI, err error) {
		if err != nil || u == nil {
			return
		}
		base := exportBaseName(item.SourceName, tr.Model)
		txtURI, err := storage.Child(u, base+".txt")
		if err != nil {
			showError(p.window, err)
			return
		}
		out, err := storage.Writer(txtURI)
		if err != nil {
			showError(p.window, err)
			return
		}
		werr := writeText(out, tr, start)
		if cerr := out.Close(); werr == nil {
			werr = cerr
		}
		if werr != nil {
			showError(p.window, werr)
			return
		}
		p.AppendLog("Exported: " + txtURI.Path())
		if item.SourcePath != "" {
			if audioOut, copyErr := copyAudioBeside(item.SourcePath, txtURI.Path()); copyErr != nil {
				p.AppendLog(fmt.Sprintf("  audio copy skipped: %v", copyErr))
			} else if audioOut != "" {
				p.AppendLog("Exported: " + audioOut)
			}
		}
	}, p.window)
	folderDialog.Show()
}
