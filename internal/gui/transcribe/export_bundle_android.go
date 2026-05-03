//go:build android

package transcribe

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	shared "github.com/asolopovas/wt/internal"
	"github.com/asolopovas/wt/internal/transcriber"
)

func (p *Panel) exportBundleFolder(tr *transcriber.Transcript, item ExportItem, start time.Time) {
	dir := shared.MediaDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		showError(p.window, err)
		return
	}

	base := exportBaseName(item.SourceName, tr.Model)
	txtPath := filepath.Join(dir, base+".txt")
	out, err := os.Create(txtPath)
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
	p.AppendLog("Exported: " + txtPath)

	audioPath := ""
	if item.SourcePath != "" {
		audioPath = filepath.Join(dir, base+filepath.Ext(item.SourcePath))
		if audioPath != item.SourcePath {
			if err := copyFileSimple(item.SourcePath, audioPath); err != nil {
				p.AppendLog(fmt.Sprintf("  audio copy skipped: %v", err))
				audioPath = ""
			} else {
				p.AppendLog("Exported: " + audioPath)
			}
		}
	}

	msg := "Saved transcript to " + dir
	if audioPath != "" {
		msg = "Saved transcript + audio to " + dir
	}
	showNotice(p.window, notifyInfo, "Export", msg)
}

func copyFileSimple(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		_ = os.Remove(dst)
		return err
	}
	return out.Close()
}
