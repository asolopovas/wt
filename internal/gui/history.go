package gui

import (
	"fmt"
	"os"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func (p *transcribePanel) attachLibrary(h *historyPanel) {
	if p.libraryHost == nil {
		return
	}
	p.libraryHost.Objects = []fyne.CanvasObject{h.container}
	p.libraryHost.Refresh()
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
	window      fyne.Window
	transcribe  *transcribePanel
	list        *fyne.Container
	empty       *canvas.Text
	container   fyne.CanvasObject
	headerRight *fyne.Container
	player      audioPlayer
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

	header := canvas.NewText("RECENT", colMuted)
	header.TextSize = 10
	header.TextStyle = fyne.TextStyle{Bold: true}

	h.headerRight = container.NewHBox()
	headerContent := container.NewBorder(nil, nil, header, h.headerRight)

	headerBar := container.NewStack(
		canvas.NewRectangle(colSurfLow),
		container.NewPadded(headerContent),
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
		meta = "fresh · added " + formatRelative(e.CreatedAt)
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
	stampText := canvas.NewText(recorded.Format(startTimeLayout), colMuted)
	stampText.TextSize = 10
	stampText.TextStyle = fyne.TextStyle{Monospace: true}
	stampText.Alignment = fyne.TextAlignTrailing

	titleRow := container.NewBorder(nil, nil, nil, stampText, name)
	info := container.NewVBox(titleRow, metaText)

	deleteBtn := newPointerButtonWithIcon("", theme.DeleteIcon(), func() {
		msg := fmt.Sprintf("Delete %s? This will remove the source file and any cached transcript.", e.SourceName)
		dialog.ShowConfirm("Delete", msg,
			func(ok bool) {
				if !ok {
					return
				}
				if h.player.playing(e.Key) {
					h.player.stop()
				}
				if e.SourcePath != "" {
					if err := os.Remove(e.SourcePath); err != nil && !os.IsNotExist(err) {
						dialog.ShowError(err, h.window)
						return
					}
				}
				if err := cacheDelete(e.Key); err != nil {
					dialog.ShowError(err, h.window)
					return
				}
				h.refresh()
			}, h.window)
	})
	deleteBtn.Importance = widget.LowImportance

	playBtn := newPointerButtonWithIcon("", playIconResource, nil)
	playBtn.Importance = widget.LowImportance
	if h.player.playing(e.Key) {
		playBtn.SetIcon(pauseIconResource)
	}
	playBtn.OnTapped = func() {
		if e.SourcePath == "" {
			dialog.ShowError(fmt.Errorf("source file path missing"), h.window)
			return
		}
		if h.player.playing(e.Key) {
			h.player.stop()
			playBtn.SetIcon(playIconResource)
			return
		}
		err := h.player.start(e.Key, e.SourcePath, func(string) {
			fyne.Do(func() { playBtn.SetIcon(playIconResource) })
		})
		if err != nil {
			dialog.ShowError(fmt.Errorf("ffplay not available: %w", err), h.window)
			return
		}
		playBtn.SetIcon(pauseIconResource)
	}

	transcribeBtn := newPointerButtonWithIcon("", transcribeIconResource, func() {
		if e.SourcePath == "" {
			dialog.ShowError(fmt.Errorf("source file path missing"), h.window)
			return
		}
		h.transcribe.startTranscription([]string{e.SourcePath})
	})
	transcribeBtn.Importance = widget.LowImportance

	editStampBtn := newPointerButtonWithIcon("", theme.HistoryIcon(), func() {
		h.editRecordedAt(e.Key, recorded)
	})
	editStampBtn.Importance = widget.LowImportance

	var actions *fyne.Container
	if e.Pending {
		actions = container.NewHBox(playBtn, transcribeBtn, editStampBtn, deleteBtn)
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

		actions = container.NewHBox(playBtn, transcribeBtn, editStampBtn, previewBtn, exportBtn, deleteBtn)
	}

	actionBg := canvas.NewRectangle(colSurfLow)
	actionRow := container.NewStack(
		actionBg,
		container.NewPadded(container.NewHBox(layout.NewSpacer(), actions)),
	)
	row := container.NewVBox(info, actionRow)

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
