package gui

import (
	"fmt"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func (p *transcribePanel) openLibrary() {
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

	body := container.NewBorder(btnRow, nil, nil, nil, p.history.container)

	dlg := dialog.NewCustom("Library", "Close", dialogBordered(body), p.window)
	dlg.Resize(libraryDialogSize(p.window))
	dlg.Show()
}

func libraryDialogSize(w fyne.Window) fyne.Size {
	cs := w.Canvas().Size()
	width := cs.Width * 0.9
	height := cs.Height * 0.85
	if width < 360 {
		width = 360
	}
	if height < 480 {
		height = 480
	}
	return fyne.NewSize(width, height)
}

type historyPanel struct {
	window     fyne.Window
	transcribe *transcribePanel
	list       *fyne.Container
	empty      *canvas.Text
	container  fyne.CanvasObject
}

func newHistoryPanel(window fyne.Window, tp *transcribePanel) *historyPanel {
	hp := &historyPanel{window: window, transcribe: tp}
	hp.build()
	hp.rebuild()
	return hp
}

func (h *historyPanel) build() {
	h.list = container.NewVBox()

	h.empty = canvas.NewText("No transcriptions yet.", colSurfBright)
	h.empty.TextSize = 11
	h.empty.Alignment = fyne.TextAlignCenter

	scroll := container.NewVScroll(h.list)

	header := canvas.NewText("RECENT TRANSCRIPTIONS", colMuted)
	header.TextSize = 10
	header.TextStyle = fyne.TextStyle{Bold: true}

	refreshBtn := newPointerButtonWithIcon("", theme.ViewRefreshIcon(), h.refresh)
	refreshBtn.Importance = widget.LowImportance

	headerBar := container.NewStack(
		canvas.NewRectangle(colSurfLow),
		container.NewPadded(container.NewHBox(header, layout.NewSpacer(), refreshBtn)),
	)

	bg := canvas.NewRectangle(colSurfLowest)
	bg.StrokeColor = colGhostBorder
	bg.StrokeWidth = 1

	h.container = container.NewStack(
		bg,
		container.NewBorder(headerBar, nil, nil, nil, scroll),
	)
}

func (h *historyPanel) refresh() {
	fyne.Do(h.rebuild)
}

func (h *historyPanel) rebuild() {
	entries := cacheEntriesByRecent()
	h.list.Objects = nil
	if len(entries) == 0 {
		h.list.Add(container.NewCenter(h.empty))
	} else {
		for _, e := range entries {
			h.list.Add(h.buildRow(e))
		}
	}
	h.list.Refresh()
}

func (h *historyPanel) buildRow(e cacheEntry) fyne.CanvasObject {
	name := canvas.NewText(e.SourceName, colForeground)
	name.TextStyle = fyne.TextStyle{Bold: true}
	name.TextSize = 12

	var meta string
	if e.Pending {
		meta = "fresh · not transcribed yet · added " + formatRelative(e.CreatedAt)
	} else {
		lang := e.Language
		if lang == "" {
			lang = "auto"
		}
		meta = fmt.Sprintf("%s · %s · %d segments · %s",
			e.Model, lang, e.Utterances, formatRelative(e.CreatedAt))
	}
	metaText := canvas.NewText(meta, colMuted)
	metaText.TextSize = 10
	metaText.TextStyle = fyne.TextStyle{Monospace: true}

	recorded := recordedAtOrFallback(e)
	stampBtn := newPointerButtonWithIcon(recorded.Format(startTimeLayout), theme.HistoryIcon(), func() {
		h.editRecordedAt(e.Key, recorded)
	})
	stampBtn.Importance = widget.LowImportance
	stampRow := container.NewHBox(stampBtn, layout.NewSpacer())

	info := container.NewVBox(name, metaText, stampRow)

	deleteBtn := newPointerButtonWithIcon("", theme.DeleteIcon(), func() {
		var msg string
		if e.Pending {
			msg = fmt.Sprintf("Remove %s from the list?", e.SourceName)
		} else {
			msg = fmt.Sprintf("Remove cached transcript for %s?", e.SourceName)
		}
		dialog.ShowConfirm("Delete", msg,
			func(ok bool) {
				if !ok {
					return
				}
				if err := cacheDelete(e.Key); err != nil {
					dialog.ShowError(err, h.window)
					return
				}
				h.refresh()
			}, h.window)
	})
	deleteBtn.Importance = widget.LowImportance

	var actions *fyne.Container
	if e.Pending {
		actions = container.NewHBox(deleteBtn)
	} else {
		previewBtn := newPointerButtonWithIcon("", theme.VisibilityIcon(), func() {
			h.transcribe.openPreview(exportItem{
				cachePath:  transcriptPathForKey(e.Key),
				sourceName: e.SourceName,
				sourcePath: e.SourcePath,
				cacheKey:   e.Key,
				recordedAt: recorded,
			}, nil)
		})
		previewBtn.Importance = widget.LowImportance

		exportBtn := newPointerButtonWithIcon("", theme.DocumentSaveIcon(), func() {
			h.transcribe.exportTranscript([]exportItem{{
				cachePath:  transcriptPathForKey(e.Key),
				sourceName: e.SourceName,
				sourcePath: e.SourcePath,
				cacheKey:   e.Key,
				recordedAt: recorded,
			}})
		})
		exportBtn.Importance = widget.LowImportance

		actions = container.NewHBox(previewBtn, exportBtn, deleteBtn)
	}

	row := container.NewBorder(nil, nil, nil, actions, info)

	rowBg := canvas.NewRectangle(colSurfLow)
	rowBg.StrokeColor = colGhostBorder
	rowBg.StrokeWidth = 1

	return container.NewStack(rowBg, container.NewPadded(row))
}

func (h *historyPanel) editRecordedAt(key string, current time.Time) {
	showDatePicker(h.window, current, func(d time.Time) {
		showTimePicker(h.window, current, func(hh, mm, ss int) {
			combined := time.Date(d.Year(), d.Month(), d.Day(), hh, mm, ss, 0, time.Local)
			if err := cacheSetRecordedAt(key, combined); err != nil {
				dialog.ShowError(err, h.window)
				return
			}
			h.refresh()
		})
	})
}

func formatRelative(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	case d < 7*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	default:
		return t.Format("2006-01-02")
	}
}
