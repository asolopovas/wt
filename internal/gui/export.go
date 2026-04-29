package gui

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
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
	sourcePath string
	cacheKey   string
	recordedAt time.Time
}

func (p *transcribePanel) openPreview(item exportItem, onClose func()) {
	tr, err := loadTranscript(item.cachePath)
	if err != nil {
		dialog.ShowError(fmt.Errorf("loading %s: %w", item.sourceName, err), p.window)
		return
	}

	start := item.recordedAt
	if start.IsZero() {
		if t, ok := fileStartTime(item.sourcePath); ok {
			start = t
		} else {
			start = time.Now()
		}
	}

	speakerOrder := []string{}
	speakerSeen := map[string]bool{}
	for _, u := range tr.Utterances {
		if !speakerSeen[u.Speaker] {
			speakerSeen[u.Speaker] = true
			speakerOrder = append(speakerOrder, u.Speaker)
		}
	}

	if p.speakerRenames == nil {
		p.speakerRenames = map[string]string{}
	}
	for k, v := range loadSpeakerRenames(item.cacheKey) {
		p.speakerRenames[k] = v
	}

	content := widget.NewRichText()
	content.Wrapping = fyne.TextWrapWord

	const initialBatch = 20
	const batchSize = 30

	buildSegment := func(u transcriber.Utterance) *widget.TextSegment {
		return &widget.TextSegment{
			Text: fmt.Sprintf("[%s] %s: %s\n",
				formatAbsoluteTimestamp(u.Start, start),
				p.displayName(u.Speaker),
				strings.TrimSpace(u.Text)),
			Style: widget.RichTextStyle{
				ColorName: theme.ColorNamePrimary,
				TextStyle: fyne.TextStyle{Monospace: true},
			},
		}
	}

	var renderToken atomic.Uint64

	render := func() {
		token := renderToken.Add(1)
		if len(tr.Utterances) == 0 {
			content.Segments = []widget.RichTextSegment{&widget.TextSegment{
				Text: "(no utterances)",
				Style: widget.RichTextStyle{
					ColorName: theme.ColorNameForeground,
					TextStyle: fyne.TextStyle{Monospace: true},
				},
			}}
			content.Refresh()
			return
		}

		first := initialBatch
		if first > len(tr.Utterances) {
			first = len(tr.Utterances)
		}
		segs := make([]widget.RichTextSegment, 0, len(tr.Utterances))
		for i := 0; i < first; i++ {
			segs = append(segs, buildSegment(tr.Utterances[i]))
		}
		content.Segments = segs
		content.Refresh()

		if first >= len(tr.Utterances) {
			return
		}

		go func(from int) {
			for i := from; i < len(tr.Utterances); i += batchSize {
				if renderToken.Load() != token {
					return
				}
				end := i + batchSize
				if end > len(tr.Utterances) {
					end = len(tr.Utterances)
				}
				chunk := make([]widget.RichTextSegment, 0, end-i)
				for j := i; j < end; j++ {
					chunk = append(chunk, buildSegment(tr.Utterances[j]))
				}
				fyne.Do(func() {
					if renderToken.Load() != token {
						return
					}
					content.Segments = append(content.Segments, chunk...)
					content.Refresh()
				})
			}
		}(first)
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
			_ = saveSpeakerRenames(item.cacheKey, p.speakerRenames)
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

	editing := false
	editBtn := newPointerButton("RENAME", nil)
	editBtn.Importance = widget.LowImportance
	editBtn.OnTapped = func() {
		editing = !editing
		if editing {
			if speakerPanel != nil {
				speakerPanel.Show()
			}
			editBtn.SetText("DONE")
		} else {
			if speakerPanel != nil {
				speakerPanel.Hide()
			}
			editBtn.SetText("RENAME")
		}
	}

	var hidePreview func()
	exportBtn := newPointerButton("EXPORT", func() {
		if hidePreview != nil {
			hidePreview()
		}
		p.exportSinglePrompt(item, start)
	})
	exportBtn.Importance = widget.LowImportance

	closeBtn := newPointerButton("CLOSE", func() {
		if hidePreview != nil {
			hidePreview()
		}
	})
	closeBtn.Importance = widget.LowImportance

	buttons := container.NewGridWithColumns(3,
		borderedBtn(closeBtn, colOutline),
		borderedBtn(exportBtn, colOutline),
		borderedBtn(editBtn, colOutline),
	)
	bottomGap := canvas.NewRectangle(transparent)
	bottomGap.SetMinSize(fyne.NewSize(0, previewBottomInset()))
	actionRow := container.NewVBox(buttons, bottomGap)

	stampText := canvas.NewText(start.Format(startTimeLayout), colMuted)
	stampText.TextSize = 11
	stampText.TextStyle = fyne.TextStyle{Monospace: true, Bold: true}
	stampLabel := canvas.NewText("RECORDED", colMuted)
	stampLabel.TextSize = 10
	stampLabel.TextStyle = fyne.TextStyle{Bold: true}
	stampRow := container.NewHBox(stampLabel, stampText)

	topGap := canvas.NewRectangle(transparent)
	topGap.SetMinSize(fyne.NewSize(0, previewTopInset()))

	var top fyne.CanvasObject = container.NewVBox(topGap, stampRow)
	if speakerPanel != nil {
		top = container.NewVBox(topGap, stampRow, speakerPanel)
	}

	render()

	buildPlainText := func() string {
		var sb strings.Builder
		for _, u := range tr.Utterances {
			fmt.Fprintf(&sb, "[%s] %s: %s\n",
				formatAbsoluteTimestamp(u.Start, start),
				p.displayName(u.Speaker),
				strings.TrimSpace(u.Text))
		}
		return sb.String()
	}

	copyBtn := newPointerButtonWithIcon("", theme.ContentCopyIcon(), nil)
	copyBtn.Importance = widget.HighImportance
	copyBtn.OnTapped = func() {
		fyne.CurrentApp().Clipboard().SetContent(buildPlainText())
		copyBtn.SetIcon(theme.ConfirmIcon())
		go func() {
			time.Sleep(900 * time.Millisecond)
			fyne.Do(func() { copyBtn.SetIcon(theme.ContentCopyIcon()) })
		}()
	}

	floating := container.New(newTopRightFloater(12, 12), copyBtn)
	bodyInner := container.NewBorder(top, actionRow, nil, nil, scroll)
	body := container.NewStack(bodyInner, floating)
	hidePreview = showTranscriptPreview(item.sourceName, body, p.window, onClose)
}

func (p *transcribePanel) exportSinglePrompt(item exportItem, start time.Time) {
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
				p.exportSingleAs(f, item, start)
				return
			}
		}
	}, p.window)
}

func itemStartTime(item exportItem) time.Time {
	if !item.recordedAt.IsZero() {
		return item.recordedAt
	}
	if t, ok := fileStartTime(item.sourcePath); ok {
		return t
	}
	return time.Now()
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
					p.exportSingleAs(f, items[0], itemStartTime(items[0]))
				} else {
					p.exportBatchAs(f, items)
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

	saveDialog.SetFileName(exportBaseName(item.sourceName, tr.Model) + "." + f.ext)
	saveDialog.SetFilter(storage.NewExtensionFileFilter([]string{"." + f.ext}))
	saveDialog.Show()
}

func (p *transcribePanel) exportBatchAs(f exportFormat, items []exportItem) {
	folderDialog := dialog.NewFolderOpen(func(u fyne.ListableURI, err error) {
		if err != nil || u == nil {
			return
		}
		var done, failed int
		for _, item := range items {
			tr, err := loadTranscript(item.cachePath)
			if err != nil {
				p.appendLog(fmt.Sprintf("Export failed for %s: %v", item.sourceName, err))
				failed++
				continue
			}
			model := tr.Model
			tr = p.renamedTranscript(tr)
			name := exportBaseName(item.sourceName, model) + "." + f.ext
			childURI, err := storage.Child(u, name)
			if err != nil {
				p.appendLog(fmt.Sprintf("Export failed for %s: %v", item.sourceName, err))
				failed++
				continue
			}
			out, err := storage.Writer(childURI)
			if err != nil {
				p.appendLog(fmt.Sprintf("Export failed for %s: %v", item.sourceName, err))
				failed++
				continue
			}
			werr := writeExport(out, tr, f, itemStartTime(item))
			_ = out.Close()
			if werr != nil {
				p.appendLog(fmt.Sprintf("Export failed for %s: %v", item.sourceName, werr))
				failed++
				continue
			}
			p.appendLog("Exported: " + childURI.Path())
			done++
		}
		if failed > 0 {
			dialog.ShowInformation("Export",
				fmt.Sprintf("Exported %d of %d. %d failed.", done, done+failed, failed), p.window)
		} else {
			dialog.ShowInformation("Export",
				fmt.Sprintf("Exported %d file(s) to %s", done, u.Path()), p.window)
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

func exportBaseName(sourceName, model string) string {
	base := strings.TrimSuffix(filepath.Base(sourceName), filepath.Ext(sourceName))
	if model = strings.TrimSpace(model); model != "" {
		base += "-" + model
	}
	return base
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

func formatAbsoluteTimestamp(ms int64, start time.Time) string {
	return start.Add(time.Duration(ms) * time.Millisecond).Format(startTimeLayout)
}

func fileStartTime(path string) (time.Time, bool) {
	if path == "" {
		return time.Time{}, false
	}
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}, false
	}
	return info.ModTime().Local(), true
}
