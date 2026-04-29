//go:build android

package gui

import (
	"fmt"
	"os"
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

func (p *transcribePanel) onOpen() {
	if p.lastCSVPath == "" {
		dialog.ShowInformation("Open", "No output yet. Transcribe a file first.", p.window)
		return
	}

	data, err := os.ReadFile(p.lastCSVPath)
	if err != nil {
		dialog.ShowError(fmt.Errorf("reading output: %w", err), p.window)
		return
	}

	content := widget.NewRichText(&widget.TextSegment{
		Text: string(data),
		Style: widget.RichTextStyle{
			TextStyle: fyne.TextStyle{Monospace: true},
		},
	})
	content.Wrapping = fyne.TextWrapWord
	scroll := container.NewVScroll(content)
	scroll.SetMinSize(fyne.NewSize(300, 400))

	dialog.ShowCustom(filepath.Base(p.lastCSVPath), "Close", scroll, p.window)
}

func openExternal(p *transcribePanel, path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		dialog.ShowError(fmt.Errorf("reading output: %w", err), p.window)
		return
	}
	content := widget.NewRichText(&widget.TextSegment{
		Text: string(data),
		Style: widget.RichTextStyle{
			TextStyle: fyne.TextStyle{Monospace: true},
		},
	})
	content.Wrapping = fyne.TextWrapWord
	scroll := container.NewVScroll(content)
	scroll.SetMinSize(fyne.NewSize(300, 400))
	dialog.ShowCustom(filepath.Base(path), "Close", scroll, p.window)
}
