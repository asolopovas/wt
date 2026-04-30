package gui

import (
	"fmt"
	"os"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/asolopovas/wt/internal/gui/cache"
	"github.com/asolopovas/wt/internal/gui/player"
	"github.com/asolopovas/wt/internal/gui/transcribe"
	"github.com/asolopovas/wt/internal/transcriber"
)

const startTimeLayout = "2006-01-02 15:04:05"

type historyPanel struct {
	window      fyne.Window
	transcribe  *transcribe.Panel
	list        *fyne.Container
	empty       *canvas.Text
	container   fyne.CanvasObject
	headerRight *fyne.Container
	player      player.Player
}

func (h *historyPanel) Container() fyne.CanvasObject {
	return h.container
}

func newHistoryPanel(window fyne.Window, tp *transcribe.Panel) *historyPanel {
	hp := &historyPanel{window: window, transcribe: tp}
	hp.build()
	hp.rebuild()
	return hp
}

func (h *historyPanel) build() {
	h.list = container.NewVBox()

	h.empty = canvas.NewText("No transcriptions yet.", colSurfBright)
	h.empty.TextSize = textBody
	h.empty.Alignment = fyne.TextAlignCenter

	scroll := container.NewVScroll(h.list)

	header := newCaptionText("RECENT")

	h.headerRight = container.NewHBox()
	headerContent := container.NewBorder(nil, nil, header, h.headerRight)

	headerBar := container.NewStack(
		canvas.NewRectangle(surfaceRaised),
		container.NewPadded(headerContent),
	)

	h.container = container.NewStack(
		newPanelBackground(),
		container.NewBorder(headerBar, nil, nil, nil, scroll),
	)
}

func (h *historyPanel) Refresh() {
	fyne.Do(h.rebuild)
}

func (h *historyPanel) rebuild() {
	entries := cache.EntriesByRecent()
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

func formatDurationCompact(ms int64) string {
	if ms <= 0 {
		return "--:--"
	}
	return transcriber.FormatHMS(time.Duration(ms) * time.Millisecond)
}

func (h *historyPanel) buildRow(e cache.Entry) fyne.CanvasObject {
	nameText := canvas.NewText(e.SourceName, colForeground)
	nameText.TextSize = textRow
	nameText.TextStyle = fyne.TextStyle{Bold: true}

	recorded := cache.RecordedAtOrFallback(e)
	metaText := canvas.NewText(
		recorded.Format(startTimeLayout)+"   "+formatDurationCompact(e.DurationMs),
		colMuted,
	)
	metaText.TextSize = textBody
	metaText.TextStyle = fyne.TextStyle{Monospace: true}

	info := container.New(&tightVBox{gap: spaceXS}, nameText, metaText)

	moreBtn := newPointerButtonWithIcon("", theme.MoreVerticalIcon(), nil)
	moreBtn.Importance = widget.LowImportance
	moreBtn.OnTapped = func() {
		h.showRowMenu(e, recorded, moreBtn)
	}

	wrap := func(btn fyne.CanvasObject) fyne.CanvasObject {
		return container.NewGridWrap(fyne.NewSize(32, 32), btn)
	}

	actions := container.NewHBox()

	if e.Pending {
		spinner := widget.NewActivity()
		spinner.Start()
		actions.Add(wrap(container.NewCenter(spinner)))
	} else {
		playBtn := newPointerButtonWithIcon("", playIconResource, nil)
		playBtn.Importance = widget.LowImportance
		if h.player.Playing(e.Key) {
			playBtn.SetIcon(pauseIconResource)
		}
		playBtn.OnTapped = func() {
			if e.SourcePath == "" {
				showError(h.window, fmt.Errorf("source file path missing"))
				return
			}
			if h.player.Playing(e.Key) {
				h.player.Stop()
				playBtn.SetIcon(playIconResource)
				return
			}
			err := h.player.Start(e.Key, e.SourcePath, func(string) {
				fyne.Do(func() { playBtn.SetIcon(playIconResource) })
			})
			if err != nil {
				showError(h.window, fmt.Errorf("ffplay not available: %w", err))
				return
			}
			playBtn.SetIcon(pauseIconResource)
		}

		previewBtn := newPointerButtonWithIcon("", theme.VisibilityIcon(), nil)
		previewBtn.Importance = widget.LowImportance
		previewBtn.OnTapped = func() {
			h.transcribe.OpenPreview(transcribe.ExportItem{
				CachePath:  cache.TranscriptPathForKey(e.Key),
				SourceName: e.SourceName,
				SourcePath: e.SourcePath,
				CacheKey:   e.Key,
				RecordedAt: recorded,
			}, nil)
		}

		actions.Add(wrap(playBtn))
		actions.Add(wrap(previewBtn))
	}

	actions.Add(wrap(moreBtn))

	row := container.NewBorder(nil, nil, nil, container.NewCenter(actions), info)

	rowBg := canvas.NewRectangle(surfaceRaised)
	rowBg.StrokeColor = borderSubtle
	rowBg.StrokeWidth = 1

	return container.NewStack(rowBg, container.NewPadded(row))
}

func (h *historyPanel) showRowMenu(e cache.Entry, recorded time.Time, anchor fyne.CanvasObject) {
	items := []*fyne.MenuItem{
		fyne.NewMenuItem("Transcribe", func() {
			if e.SourcePath == "" {
				showError(h.window, fmt.Errorf("source file path missing"))
				return
			}
			h.transcribe.StartTranscription([]string{e.SourcePath})
		}),
	}

	if !e.Pending {
		items = append(items,
			fyne.NewMenuItem("Preview", func() {
				h.transcribe.OpenPreview(transcribe.ExportItem{
					CachePath:  cache.TranscriptPathForKey(e.Key),
					SourceName: e.SourceName,
					SourcePath: e.SourcePath,
					CacheKey:   e.Key,
					RecordedAt: recorded,
				}, nil)
			}),
			fyne.NewMenuItem("Export", func() {
				h.transcribe.ExportTranscript([]transcribe.ExportItem{{
					CachePath:  cache.TranscriptPathForKey(e.Key),
					SourceName: e.SourceName,
					SourcePath: e.SourcePath,
					CacheKey:   e.Key,
					RecordedAt: recorded,
				}})
			}),
		)
	}

	items = append(items,
		fyne.NewMenuItem("Edit timestamp", func() {
			h.editRecordedAt(e.Key, recorded)
		}),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Delete", func() {
			msg := fmt.Sprintf("Delete %s? This will remove the source file and any cached transcript.", e.SourceName)
			showConfirm(h.window, "Delete", msg, func() {
				if h.player.Playing(e.Key) {
					h.player.Stop()
				}
				if e.SourcePath != "" {
					if err := os.Remove(e.SourcePath); err != nil && !os.IsNotExist(err) {
						showError(h.window, err)
						return
					}
				}
				if err := cache.Delete(e.Key); err != nil {
					showError(h.window, err)
					return
				}
				h.Refresh()
			})
		}),
	)

	c := fyne.CurrentApp().Driver().CanvasForObject(anchor)
	if c == nil {
		return
	}
	pop := widget.NewPopUpMenu(fyne.NewMenu("", items...), c)
	pos := fyne.CurrentApp().Driver().AbsolutePositionForObject(anchor)
	pos = pos.Add(fyne.NewPos(anchor.Size().Width-pop.MinSize().Width, anchor.Size().Height))
	pop.ShowAtPosition(pos)
}

func (h *historyPanel) editRecordedAt(key string, current time.Time) {
	showDatePicker(h.window, current, func(d time.Time) {
		showTimePicker(h.window, current, func(hh, mm, ss int) {
			combined := time.Date(d.Year(), d.Month(), d.Day(), hh, mm, ss, 0, time.Local)
			if err := cache.SetRecordedAt(key, combined); err != nil {
				showError(h.window, err)
				return
			}
			h.Refresh()
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
