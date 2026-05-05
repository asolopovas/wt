package gui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	shared "github.com/asolopovas/wt/internal"
	"github.com/asolopovas/wt/internal/gui/decor"
	"github.com/asolopovas/wt/internal/gui/player"
	"github.com/asolopovas/wt/internal/gui/transcribe"
	"github.com/asolopovas/wt/internal/llm"
	"github.com/asolopovas/wt/internal/models"
	"github.com/asolopovas/wt/internal/namer"
	"github.com/asolopovas/wt/internal/transcriber"
	"github.com/asolopovas/wt/internal/transcriber/cache"
)

var llmDownloadOnce sync.Mutex

const startTimeLayout = "2006-01-02 15:04:05"

type historyPanel struct {
	window      fyne.Window
	transcribe  *transcribe.Panel
	list        *fyne.Container
	empty       *canvas.Text
	container   fyne.CanvasObject
	headerRight *fyne.Container
	player      player.Player
	dock        *playerDock
}

func (h *historyPanel) Container() fyne.CanvasObject {
	return h.container
}

func newHistoryPanel(window fyne.Window, tp *transcribe.Panel, dock *playerDock) *historyPanel {
	hp := &historyPanel{window: window, transcribe: tp, dock: dock}
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

	h.headerRight = container.NewHBox()
	headerBar := decor.NewPanelHeader(newCaptionText("RECENT"), h.headerRight)

	h.container = container.NewBorder(headerBar, nil, nil, nil, scroll)
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
	nameText := newTruncText(e.SourceName, colForeground, textRow, fyne.TextStyle{Bold: true})

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

	rowIcon := playIconResource
	if h.dock != nil {
		rowIcon = editAudioIcon
	}
	playBtn := newPointerButtonWithIcon("", rowIcon, nil)
	playBtn.Importance = widget.LowImportance
	if h.dock == nil && h.player.Playing(e.Key) {
		playBtn.SetIcon(pauseIconResource)
	}
	playBtn.OnTapped = func() {
		if e.SourcePath == "" {
			showError(h.window, fmt.Errorf("source file path missing"))
			return
		}
		if h.dock != nil {

			h.dock.Load(e.Key, e.SourcePath, e.SourceName, false)
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
	actions.Add(wrap(playBtn))

	if e.Pending {
		if h.transcribe != nil && h.transcribe.IsActivePath(e.SourcePath) {
			spinner := widget.NewActivity()
			spinner.Start()
			actions.Add(wrap(container.NewCenter(spinner)))
		} else {
			actions.Add(wrap(container.NewCenter(canvas.NewText("…", colMuted))))
		}
	} else {
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
		items = append(
			items,
			fyne.NewMenuItem("Export", func() {
				h.transcribe.ExportTranscript([]transcribe.ExportItem{{
					CachePath:  cache.TranscriptPathForKey(e.Key),
					SourceName: e.SourceName,
					SourcePath: e.SourcePath,
					CacheKey:   e.Key,
					RecordedAt: recorded,
				}})
			}),
			fyne.NewMenuItem("Share", func() {
				h.transcribe.ShareTranscript([]transcribe.ExportItem{{
					CachePath:  cache.TranscriptPathForKey(e.Key),
					SourceName: e.SourceName,
					SourcePath: e.SourcePath,
					CacheKey:   e.Key,
					RecordedAt: recorded,
				}})
			}),
			fyne.NewMenuItem("Re-diarize", func() {
				if e.SourcePath == "" {
					showError(h.window, fmt.Errorf("source file path missing"))
					return
				}
				if err := cache.InvalidateTranscript(e.Key); err != nil {
					showError(h.window, fmt.Errorf("invalidating cache: %w", err))
					return
				}
				h.transcribe.StartTranscription([]string{e.SourcePath})
			}),
		)
	}

	items = append(
		items,
		fyne.NewMenuItem("Rename", func() {
			h.renameEntry(e)
		}),
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

func (h *historyPanel) renameEntry(e cache.Entry) {
	ext := filepath.Ext(e.SourceName)
	stem := strings.TrimSuffix(e.SourceName, ext)

	entry := widget.NewEntry()
	entry.SetText(stem)

	entryThemed := container.NewThemeOverride(entry, &renameEntryTheme{parent: fyne.CurrentApp().Settings().Theme()})
	entryScroll := container.NewHScroll(entryThemed)
	entrySized := container.New(&fixedHeightLayout{height: 56}, entryScroll)

	clipboard := fyne.CurrentApp().Clipboard()
	cutBtn := newPointerButtonWithIcon("", theme.ContentCutIcon(), func() {
		entry.TypedShortcut(&fyne.ShortcutCut{Clipboard: clipboard})
	})
	cutBtn.Importance = widget.LowImportance
	copyBtn := newPointerButtonWithIcon("", theme.ContentCopyIcon(), func() {
		entry.TypedShortcut(&fyne.ShortcutCopy{Clipboard: clipboard})
	})
	copyBtn.Importance = widget.LowImportance
	pasteBtn := newPointerButtonWithIcon("", theme.ContentPasteIcon(), func() {
		entry.TypedShortcut(&fyne.ShortcutPaste{Clipboard: clipboard})
	})
	pasteBtn.Importance = widget.LowImportance

	status := widget.NewLabel("")
	status.Wrapping = fyne.TextWrapWord
	status.TextStyle = fyne.TextStyle{Italic: true}

	setStatus := func(msg string) {
		status.SetText(msg)
	}

	autoBtn := newPointerButton("AUTO-RENAME", func() {
		setStatus("Generating…")
		go func() {
			stem, serr := h.suggestStemForEntry(e)
			fyne.Do(func() {
				switch {
				case serr == nil:
					entry.SetText(stem)
					setStatus("")
				case errors.Is(serr, llm.ErrNoLLMInstalled):
					setStatus("Downloading LLM… try again shortly.")
					go h.ensureLLMDownload()
				default:
					shared.LogError(fmt.Sprintf("Auto-rename failed: %v", serr))
					msg := serr.Error()
					if len(msg) > 80 {
						msg = msg[:80] + "…"
					}
					setStatus("Auto-rename failed: " + msg + " (see wt.log)")
				}
			})
		}()
	})
	autoBtn.Importance = widget.LowImportance

	toolbar := container.NewHBox(autoBtn, layout.NewSpacer(), cutBtn, copyBtn, pasteBtn)

	form := container.New(&tightVBox{gap: spaceSM}, entrySized, toolbar)

	showDialog(dialogConfig{
		Parent:     h.window,
		Title:      "RENAME",
		TitleRight: status,
		Body:       form,
		AnchorTop:  true,
		WidthFrac:  0.85,
		Actions: []dialogAction{
			{Label: "CANCEL", Kind: kindSecondary},
			{Label: "SAVE", Kind: kindPrimary, OnTap: func() {
				newStem := strings.TrimSpace(entry.Text)
				if newStem == "" || newStem == stem {
					return
				}
				newName := newStem + ext
				newPath := e.SourcePath
				if e.SourcePath != "" {
					newPath = filepath.Join(filepath.Dir(e.SourcePath), newName)
					if newPath != e.SourcePath {
						if err := os.Rename(e.SourcePath, newPath); err != nil && !os.IsNotExist(err) {
							showError(h.window, fmt.Errorf("renaming file: %w", err))
							return
						}
					}
					newName = filepath.Base(newPath)
				}
				if err := cache.SetSource(e.Key, newPath, newName); err != nil {
					showError(h.window, fmt.Errorf("updating cache: %w", err))
					return
				}
				h.Refresh()
			}},
		},
	})

	fyne.Do(func() {
		c := h.window.Canvas()
		if c == nil {
			return
		}
		c.Focus(entry)
		entry.TypedShortcut(&fyne.ShortcutSelectAll{})
	})
}

func (h *historyPanel) ensureLLMDownload() {
	if !llmDownloadOnce.TryLock() {
		return
	}
	defer llmDownloadOnce.Unlock()

	mgr := models.NewManager()
	var target models.Entry
	found := false
	for _, e := range models.ByFamily(models.FamilyLLM) {
		if e.DefaultActive {
			target = e
			found = true
			break
		}
	}
	if !found {
		return
	}
	if mgr.Status(target.ID) == models.StatusInstalled {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	_ = mgr.Get(ctx, target.ID, nil)
}

func (h *historyPanel) suggestStemForEntry(e cache.Entry) (string, error) {
	jsonPath := cache.TranscriptPathForKey(e.Key)
	text, err := namer.ExtractTranscriptText(jsonPath)
	if err != nil {
		return "", fmt.Errorf("reading transcript: %w", err)
	}
	if strings.TrimSpace(text) == "" {
		return "", fmt.Errorf("transcript is empty")
	}
	fallback := time.Time{}
	if e.SourcePath != "" {
		if st, statErr := os.Stat(e.SourcePath); statErr == nil {
			fallback = st.ModTime()
		}
	}
	if fallback.IsZero() {
		fallback = e.CreatedAt
	}
	if fallback.IsZero() {
		fallback = time.Now()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	s, err := namer.Suggest(ctx, text, fallback)
	if err != nil {
		return "", err
	}
	name := s.Filename("")
	return name, nil
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
