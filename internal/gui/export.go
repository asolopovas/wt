package gui

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"
	"github.com/xuri/excelize/v2"

	"github.com/asolopovas/wt/internal/transcriber"
)

type exportFormat struct {
	label string
	ext   string
}

var exportFormats = []exportFormat{
	{"JSON", "json"},
	{"CSV", "csv"},
	{"XLSX", "xlsx"},
	{"Text", "txt"},
}

type exportItem struct {
	cachePath  string
	sourceName string
}

func (p *transcribePanel) onExport() {
	p.exportTranscript(p.results)
}

func (p *transcribePanel) onPreview() {
	if len(p.results) == 0 {
		dialog.ShowInformation("Preview", "No output yet. Transcribe a file first.", p.window)
		return
	}

	start, err := p.resolveStartTime()
	if err != nil {
		dialog.ShowError(err, p.window)
		return
	}

	transcripts := make(map[string]*transcriber.Transcript, len(p.results))
	speakerOrder := []string{}
	speakerSeen := map[string]bool{}
	for _, it := range p.results {
		tr, err := loadTranscript(it.cachePath)
		if err != nil {
			dialog.ShowError(fmt.Errorf("loading %s: %w", it.sourceName, err), p.window)
			return
		}
		transcripts[it.cachePath] = tr
		for _, u := range tr.Utterances {
			if !speakerSeen[u.Speaker] {
				speakerSeen[u.Speaker] = true
				speakerOrder = append(speakerOrder, u.Speaker)
			}
		}
	}

	if p.speakerRenames == nil {
		p.speakerRenames = map[string]string{}
	}

	content := widget.NewMultiLineEntry()
	content.Wrapping = fyne.TextWrapWord
	content.TextStyle = fyne.TextStyle{Monospace: true}
	content.Disable()

	var currentItem exportItem
	render := func() {
		tr := transcripts[currentItem.cachePath]
		if tr == nil {
			content.SetText("")
			return
		}
		var sb strings.Builder
		for _, u := range tr.Utterances {
			sb.WriteString(fmt.Sprintf("[%s] %s: %s\n",
				formatAbsoluteTimestamp(u.Start, start),
				p.displayName(u.Speaker),
				strings.TrimSpace(u.Text)))
		}
		if sb.Len() == 0 {
			sb.WriteString("(no utterances)")
		}
		content.SetText(sb.String())
	}

	speakerRows := make([]fyne.CanvasObject, 0, len(speakerOrder))
	for _, sp := range speakerOrder {
		speaker := sp
		entry := widget.NewEntry()
		entry.PlaceHolder = speaker
		if alias, ok := p.speakerRenames[speaker]; ok {
			entry.SetText(alias)
		}
		entry.OnChanged = func(s string) {
			if strings.TrimSpace(s) == "" {
				delete(p.speakerRenames, speaker)
			} else {
				p.speakerRenames[speaker] = s
			}
			render()
		}
		label := canvas.NewText(speaker, colMuted)
		label.TextSize = 11
		label.TextStyle = fyne.TextStyle{Monospace: true, Bold: true}
		speakerRows = append(speakerRows,
			container.NewBorder(nil, nil, container.NewGridWrap(fyne.NewSize(130, 30), container.NewCenter(label)), nil, entry))
	}

	var speakerPanel *fyne.Container
	if len(speakerRows) > 0 {
		header := canvas.NewText("SPEAKERS (edit to rename)", colMuted)
		header.TextSize = 10
		header.TextStyle = fyne.TextStyle{Bold: true}
		speakerPanel = container.NewVBox(append([]fyne.CanvasObject{header}, speakerRows...)...)
		speakerPanel.Hide()
	}

	scroll := container.NewScroll(content)
	scroll.SetMinSize(previewScrollMinSize())

	currentItem = p.results[0]
	var picker *widget.Select
	if len(p.results) > 1 {
		names := make([]string, len(p.results))
		for i, it := range p.results {
			names[i] = it.sourceName
		}
		picker = widget.NewSelect(names, func(sel string) {
			for _, it := range p.results {
				if it.sourceName == sel {
					currentItem = it
					render()
					return
				}
			}
		})
		picker.SetSelected(names[0])
	}

	editing := false
	editBtn := widget.NewButton("EDIT", nil)
	editBtn.OnTapped = func() {
		editing = !editing
		if editing {
			content.Enable()
			if speakerPanel != nil {
				speakerPanel.Show()
			}
			editBtn.SetText("DONE")
		} else {
			content.Disable()
			if speakerPanel != nil {
				speakerPanel.Hide()
			}
			editBtn.SetText("EDIT")
		}
	}

	topItems := []fyne.CanvasObject{}
	if picker != nil {
		topItems = append(topItems, picker)
	}
	topItems = append(topItems, container.NewBorder(nil, nil, nil, editBtn, widget.NewLabel("")))
	if speakerPanel != nil {
		topItems = append(topItems, speakerPanel)
	}
	top := container.NewVBox(topItems...)

	render()

	body := container.NewBorder(top, nil, nil, nil, scroll)
	showTranscriptPreview("Transcript preview", body, p.window)
}

func (p *transcribePanel) renamedTranscript(tr *transcriber.Transcript) *transcriber.Transcript {
	if len(p.speakerRenames) == 0 {
		return tr
	}
	cp := *tr
	cp.Utterances = make([]transcriber.Utterance, len(tr.Utterances))
	for i, u := range tr.Utterances {
		renamed := u
		renamed.Speaker = p.displayName(u.Speaker)
		cp.Utterances[i] = renamed
	}
	cp.Words = make([]transcriber.Word, len(tr.Words))
	for i, w := range tr.Words {
		renamed := w
		renamed.Speaker = p.displayName(w.Speaker)
		cp.Words[i] = renamed
	}
	return &cp
}

func (p *transcribePanel) exportTranscript(items []exportItem) {
	if len(items) == 0 {
		dialog.ShowInformation("Export", "No output yet. Transcribe a file first.", p.window)
		return
	}
	for _, it := range items {
		if _, err := os.Stat(it.cachePath); err != nil {
			dialog.ShowError(fmt.Errorf("output file not found: %s", it.cachePath), p.window)
			return
		}
	}

	start, err := p.resolveStartTime()
	if err != nil {
		dialog.ShowError(err, p.window)
		return
	}

	labels := make([]string, len(exportFormats))
	for i, f := range exportFormats {
		labels[i] = f.label
	}
	radio := widget.NewRadioGroup(labels, nil)
	radio.SetSelected(exportFormats[0].label)

	dialog.ShowCustomConfirm("Export format", "Export", "Cancel", radio, func(ok bool) {
		if !ok {
			return
		}
		for _, f := range exportFormats {
			if f.label == radio.Selected {
				if len(items) == 1 {
					p.exportSingleAs(f, items[0], start)
				} else {
					p.exportBatchAs(f, items, start)
				}
				return
			}
		}
	}, p.window)
}

func (p *transcribePanel) exportSingleAs(f exportFormat, item exportItem, start time.Time) {
	tr, err := loadTranscript(item.cachePath)
	if err != nil {
		dialog.ShowError(err, p.window)
		return
	}
	tr = p.renamedTranscript(tr)

	saveDialog := dialog.NewFileSave(func(w fyne.URIWriteCloser, err error) {
		if err != nil || w == nil {
			return
		}
		defer func() { _ = w.Close() }()
		if writeErr := writeExport(w, tr, f, start); writeErr != nil {
			dialog.ShowError(writeErr, p.window)
			return
		}
		p.appendLog("Exported: " + w.URI().Path())
	}, p.window)

	saveDialog.SetFileName(exportBaseName(item.sourceName) + "." + f.ext)
	saveDialog.SetFilter(storage.NewExtensionFileFilter([]string{"." + f.ext}))
	saveDialog.Show()
}

func (p *transcribePanel) exportBatchAs(f exportFormat, items []exportItem, start time.Time) {
	folderDialog := dialog.NewFolderOpen(func(u fyne.ListableURI, err error) {
		if err != nil || u == nil {
			return
		}
		dir := u.Path()
		var done, failed int
		for _, item := range items {
			tr, err := loadTranscript(item.cachePath)
			if err != nil {
				p.appendLog(fmt.Sprintf("Export failed for %s: %v", item.sourceName, err))
				failed++
				continue
			}
			tr = p.renamedTranscript(tr)
			outPath := filepath.Join(dir, exportBaseName(item.sourceName)+"."+f.ext)
			out, err := os.Create(outPath)
			if err != nil {
				p.appendLog(fmt.Sprintf("Export failed for %s: %v", item.sourceName, err))
				failed++
				continue
			}
			werr := writeExport(out, tr, f, start)
			_ = out.Close()
			if werr != nil {
				p.appendLog(fmt.Sprintf("Export failed for %s: %v", item.sourceName, werr))
				failed++
				continue
			}
			p.appendLog("Exported: " + outPath)
			done++
		}
		if failed > 0 {
			dialog.ShowInformation("Export",
				fmt.Sprintf("Exported %d of %d. %d failed.", done, done+failed, failed), p.window)
		} else {
			dialog.ShowInformation("Export",
				fmt.Sprintf("Exported %d file(s) to %s", done, dir), p.window)
		}
	}, p.window)
	folderDialog.Show()
}

func loadTranscript(path string) (*transcriber.Transcript, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading output: %w", err)
	}
	var tr transcriber.Transcript
	if err := json.Unmarshal(data, &tr); err != nil {
		return nil, fmt.Errorf("parsing transcript: %w", err)
	}
	return &tr, nil
}

func exportBaseName(sourceName string) string {
	return strings.TrimSuffix(filepath.Base(sourceName), filepath.Ext(sourceName))
}

func writeExport(w io.Writer, tr *transcriber.Transcript, f exportFormat, start time.Time) error {
	switch f.ext {
	case "json":
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(tr)
	case "csv":
		return writeCSV(w, tr, start)
	case "txt":
		return writeText(w, tr, start)
	case "xlsx":
		return writeXLSX(w, tr, start)
	}
	return fmt.Errorf("unknown format: %s", f.ext)
}

func writeCSV(w io.Writer, tr *transcriber.Transcript, start time.Time) error {
	cw := csv.NewWriter(w)
	if err := cw.Write([]string{"start", "end", "speaker", "text"}); err != nil {
		return err
	}
	for _, u := range tr.Utterances {
		row := []string{
			formatAbsoluteTimestamp(u.Start, start),
			formatAbsoluteTimestamp(u.End, start),
			u.Speaker,
			strings.TrimSpace(u.Text),
		}
		if err := cw.Write(row); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

func writeText(w io.Writer, tr *transcriber.Transcript, start time.Time) error {
	for _, u := range tr.Utterances {
		line := fmt.Sprintf("[%s] %s: %s\n",
			formatAbsoluteTimestamp(u.Start, start), u.Speaker, strings.TrimSpace(u.Text))
		if _, err := io.WriteString(w, line); err != nil {
			return err
		}
	}
	return nil
}

func writeXLSX(w io.Writer, tr *transcriber.Transcript, start time.Time) error {
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()

	sheet := "Transcript"
	if err := f.SetSheetName("Sheet1", sheet); err != nil {
		return err
	}

	headers := []string{"Start", "End", "Speaker", "Text"}
	for i, h := range headers {
		cell, err := excelize.CoordinatesToCellName(i+1, 1)
		if err != nil {
			return err
		}
		if err := f.SetCellValue(sheet, cell, h); err != nil {
			return err
		}
	}

	for i, u := range tr.Utterances {
		row := i + 2
		values := []any{
			formatAbsoluteTimestamp(u.Start, start),
			formatAbsoluteTimestamp(u.End, start),
			u.Speaker,
			strings.TrimSpace(u.Text),
		}
		for col, v := range values {
			cell, err := excelize.CoordinatesToCellName(col+1, row)
			if err != nil {
				return err
			}
			if err := f.SetCellValue(sheet, cell, v); err != nil {
				return err
			}
		}
	}

	if err := f.SetColWidth(sheet, "A", "C", 22); err != nil {
		return err
	}
	if err := f.SetColWidth(sheet, "D", "D", 80); err != nil {
		return err
	}

	return f.Write(w)
}

func formatTimestamp(ms int64) string {
	return transcriber.FormatHMS(time.Duration(ms) * time.Millisecond)
}

func formatAbsoluteTimestamp(ms int64, start time.Time) string {
	return start.Add(time.Duration(ms) * time.Millisecond).Format(startTimeLayout)
}
