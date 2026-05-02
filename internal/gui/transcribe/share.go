package transcribe

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"fyne.io/fyne/v2/widget"

	"github.com/asolopovas/wt/internal/gui/platsvc"
)

// ShareTranscript opens a format picker, renders the transcript into the
// chosen format, and hands it to the OS share sheet (Android) or falls back
// to the normal export flow on desktop.
func (p *Panel) ShareTranscript(items []ExportItem) {
	if len(items) == 0 {
		showNotice(p.window, notifyInfo, "Share", "No transcripts to share yet.")
		return
	}
	if len(items) > 1 {
		// Multi-share is uncommon and confusing in chooser UIs. Defer to export.
		p.ExportTranscript(items)
		return
	}
	item := items[0]
	if _, err := os.Stat(item.CachePath); err != nil {
		showError(p.window, fmt.Errorf("transcript not found: %s", item.CachePath))
		return
	}
	if !platsvc.ShareSupported() {
		// Desktop: no native share sheet; export-to-file is the closest fit.
		p.ExportTranscript(items)
		return
	}

	labels := make([]string, len(exportFormats))
	for i, f := range exportFormats {
		labels[i] = f.label
	}
	radio := widget.NewRadioGroup(labels, nil)
	radio.SetSelected(exportFormats[0].label)

	showDialog(dialogConfig{
		Parent: p.window,
		Title:  "SHARE FORMAT",
		Body:   radio,
		Actions: []dialogAction{
			{Label: "CANCEL", Kind: kindSecondary},
			{Label: "SHARE", Kind: kindPrimary, OnTap: func() {
				for _, f := range exportFormats {
					if f.label == radio.Selected {
						p.shareSingleAs(f, item, itemStartTime(item))
						return
					}
				}
			}},
		},
		WidthFrac: 0.4,
	})
}

func (p *Panel) shareSingleAs(f exportFormat, item ExportItem, start time.Time) {
	tr, err := loadTranscript(item.CachePath)
	if err != nil {
		showError(p.window, err)
		return
	}
	tr = p.renamedTranscript(tr)

	// Plain text → share via EXTRA_TEXT (no file needed; ideal for WhatsApp).
	if runtime.GOOS == "android" && f.ext == "txt" {
		var buf bytes.Buffer
		if err := writeText(&buf, tr, start); err != nil {
			showError(p.window, err)
			return
		}
		text := strings.TrimRight(buf.String(), "\n")
		if text == "" {
			showNotice(p.window, notifyInfo, "Share", "Transcript is empty.")
			return
		}
		subject := exportBaseName(item.SourceName, tr.Model)
		go func() {
			if err := platsvc.ShareText(text, subject); err != nil {
				p.AppendLog(fmt.Sprintf("Share failed: %v", err))
				showError(p.window, err)
			}
		}()
		return
	}

	// All other formats: stage a real file in cache and hand a content:// URI to
	// the share sheet. zip bundles audio + transcripts.
	base := exportBaseName(item.SourceName, tr.Model)
	tmpDir, err := os.MkdirTemp("", "wt-share-*")
	if err != nil {
		showError(p.window, err)
		return
	}
	stagedPath := filepath.Join(tmpDir, base+"."+f.ext)
	out, err := os.Create(stagedPath)
	if err != nil {
		_ = os.RemoveAll(tmpDir)
		showError(p.window, err)
		return
	}
	if f.ext == "zip" {
		err = writeBundleZip(out, tr, item, start)
	} else {
		err = writeExport(out, tr, f, start)
	}
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		_ = os.RemoveAll(tmpDir)
		showError(p.window, err)
		return
	}

	mime := mimeForExt(f.ext)
	subject := base
	go func() {
		defer func() { _ = os.RemoveAll(tmpDir) }()
		if !platsvc.ShareSupported() {
			return
		}
		if err := platsvc.ShareFile(stagedPath, mime, subject); err != nil {
			p.AppendLog(fmt.Sprintf("Share failed: %v", err))
			showError(p.window, err)
		}
	}()
}

func mimeForExt(ext string) string {
	switch strings.ToLower(strings.TrimPrefix(ext, ".")) {
	case "txt":
		return "text/plain"
	case "csv":
		return "text/csv"
	case "json":
		return "application/json"
	case "xlsx":
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	case "zip":
		return "application/zip"
	}
	return "application/octet-stream"
}
