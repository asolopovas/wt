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
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	shared "github.com/asolopovas/wt/internal"
)

var (
	audioExtensionList = append(baseAudioExtensions, ".mp4", ".mov", ".webm", ".mkv")
	audioExtensions    = extensionSet(audioExtensionList)
)

func (p *transcribePanel) build() {
	chipsFlow := newFlowLayout(6)
	p.fileChips = container.New(chipsFlow)
	chipsFlow.setParent(p.fileChips)

	p.dropText = canvas.NewText("No files added", colMuted)
	p.dropText.TextSize = 11
	p.dropText.TextStyle = fyne.TextStyle{Monospace: true}

	p.clearBtn = newPointerButton("CLEAR ALL", p.onClear)
	p.clearCacheBtn = newPointerButton("CLEAR CACHE", p.onClearCache)
	p.transcribeBtn = newPointerButton("TRANSCRIBE", p.onTranscribe)
	p.transcribeBtn.Importance = widget.HighImportance

	p.progress = newThinProgress()

	p.statusText = canvas.NewText("READY", colMuted)
	p.statusText.TextSize = 11
	p.statusText.TextStyle = fyne.TextStyle{Monospace: true, Bold: true}

	p.timerText = canvas.NewText("", colMuted)
	p.timerText.TextSize = 11
	p.timerText.TextStyle = fyne.TextStyle{Monospace: true, Bold: true}
	p.timerText.Alignment = fyne.TextAlignTrailing

	p.statsLine = widget.NewLabel("")
	p.statsLine.TextStyle = fyne.TextStyle{Monospace: true}

	p.logText = widget.NewRichText()
	p.logText.Wrapping = fyne.TextWrapWord
	appendLogInit(p.logText)

	p.logScroll = container.NewVScroll(p.logText)

	copyBtn := newPointerButtonWithIcon("", theme.ContentCopyIcon(), p.onCopyLog)
	copyBtn.Importance = widget.LowImportance

	clearLogBtn := newPointerButtonWithIcon("", theme.HistoryIcon(), p.onClearLog)
	clearLogBtn.Importance = widget.LowImportance

	p.autoScroll.Store(true)
	p.autoBtn = newPointerButtonWithIcon("", theme.MoveDownIcon(), nil)
	p.autoBtn.Importance = widget.HighImportance
	p.autoBtn.OnTapped = p.toggleAutoScroll

	logPanel := buildLogPanel(p.logScroll, p.statsLine, copyBtn, clearLogBtn, p.autoBtn)

	p.container = logPanel
}

func (p *transcribePanel) buildFilesTab() fyne.CanvasObject {
	addBtn := newPointerButton("ADD FILES", p.onBrowse)
	addBtn.Importance = widget.HighImportance

	clearBtn := newPointerButton("CLEAR ALL", func() {
		p.files = nil
		p.rebuildChips()
		p.updateDropLabel()
	})
	clearBtn.Importance = widget.LowImportance

	btnRow := container.NewGridWithColumns(2,
		borderedBtn(addBtn, colPrimary),
		borderedBtn(clearBtn, colOutline),
	)

	filesLabel := canvas.NewText("SELECTED FILES", colMuted)
	filesLabel.TextSize = 10
	filesLabel.TextStyle = fyne.TextStyle{Bold: true}

	chipsScroll := container.NewVScroll(container.NewVBox(p.fileChips, p.dropText))

	filesBg := canvas.NewRectangle(colSurfLowest)
	filesBg.StrokeColor = colGhostBorder
	filesBg.StrokeWidth = 1

	filesInner := container.NewBorder(filesLabel, nil, nil, nil, chipsScroll)
	filesPanel := container.NewStack(filesBg, container.NewPadded(filesInner))

	bottomGap := canvas.NewRectangle(transparent)
	bottomGap.SetMinSize(fyne.NewSize(0, 6))

	return container.NewBorder(
		nil, container.NewVBox(btnRow, bottomGap), nil, nil,
		filesPanel,
	)
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
