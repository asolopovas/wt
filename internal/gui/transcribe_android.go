//go:build android

package gui

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"

	shared "github.com/asolopovas/wt/internal"
)

var (
	audioExtensionList = append(baseAudioExtensions, ".mp4", ".mov", ".webm", ".mkv")
	audioExtensions    = extensionSet(audioExtensionList)
)

func (p *transcribePanel) build() {
	chipsFlow := newFlowLayout(spaceMD)
	p.fileChips = container.New(chipsFlow)
	chipsFlow.setParent(p.fileChips)

	p.dropText = canvas.NewText("No files added", colMuted)
	p.dropText.TextSize = textBody
	p.dropText.TextStyle = fyne.TextStyle{Monospace: true}

	p.libraryHost = container.NewStack()
	p.dropArea = container.NewStack(newPanelBackground(), container.NewPadded(p.libraryHost))

	p.buildSharedControls()

	p.container = p.dropArea
}

func (p *transcribePanel) onBrowse() {
	fd := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
		if err != nil || reader == nil {
			return
		}

		localPath, copyErr := copyURIToLocal(reader)
		_ = reader.Close()
		if copyErr != nil {
			p.appendLog(fmt.Sprintf("Error importing file: %v", copyErr))
			return
		}

		if p.addLocalFile(localPath) {
			p.rebuildChips()
			p.updateDropLabel()
		}
	}, p.window)

	fd.SetFilter(storage.NewExtensionFileFilter(audioExtensionList))
	fd.Show()
}

func (p *transcribePanel) updateDropLabel() {
	if len(p.files) > 0 {
		p.dropText.Text = fmt.Sprintf("%d file(s) selected", len(p.files))
	} else {
		p.dropText.Text = "No files added"
	}
	p.dropText.Refresh()
}

func copyURIToLocal(reader fyne.URIReadCloser) (string, error) {
	uri := reader.URI()
	name := uri.Name()
	if name == "" {
		name = filepath.Base(uri.String())
	}

	cacheDir := filepath.Join(shared.CacheDir(), "imports")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return "", fmt.Errorf("creating import cache: %w", err)
	}

	dst := filepath.Join(cacheDir, name)

	f, err := os.Create(dst)
	if err != nil {
		return "", fmt.Errorf("creating local file: %w", err)
	}

	if _, err := io.Copy(f, reader); err != nil {
		_ = f.Close()
		_ = os.Remove(dst)
		return "", fmt.Errorf("copying file data: %w", err)
	}

	if err := f.Close(); err != nil {
		_ = os.Remove(dst)
		return "", fmt.Errorf("closing file: %w", err)
	}

	return dst, nil
}
