package transcribe

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/asolopovas/wt/internal/gui/platsvc"
	"github.com/asolopovas/wt/internal/llm"
	"github.com/asolopovas/wt/internal/namer"
	"github.com/asolopovas/wt/internal/transcriber"
	"github.com/asolopovas/wt/internal/transcriber/cache"
)

type renameDecision struct {
	keep    bool
	newName string
}

func (p *Panel) promptRename(originalName, suggested string, regenerate func() (string, error)) renameDecision {
	if p.window == nil {
		return renameDecision{newName: suggested}
	}

	info := widget.NewLabel("Original: " + originalName)
	info.Wrapping = fyne.TextWrapWord

	entry := widget.NewEntry()
	entry.SetText(suggested)

	hint := "Edit the suggested name, regenerate with AUTO-RENAME, or keep the original."
	caption := newCaptionText(hint)

	body := container.NewVBox(info, entry, caption)

	ch := make(chan renameDecision, 1)
	send := func(d renameDecision) {
		select {
		case ch <- d:
		default:
		}
	}

	actions := []dialogAction{
		{Label: "KEEP ORIGINAL", Kind: kindSecondary, OnTap: func() { send(renameDecision{keep: true}) }},
	}
	if regenerate != nil {
		actions = append(actions, dialogAction{Label: "AUTO-RENAME", Kind: kindSecondary, KeepOpen: true, OnTap: func() {
			caption.Text = "Regenerating…"
			caption.Refresh()
			go func() {
				suggestion, err := regenerate()
				fyne.Do(func() {
					if err != nil {
						caption.Text = "Regenerate failed: " + err.Error()
					} else {
						entry.SetText(suggestion)
						caption.Text = hint
					}
					caption.Refresh()
				})
			}()
		}})
	}
	actions = append(actions, dialogAction{Label: "RENAME", Kind: kindPrimary, OnTap: func() {
		send(renameDecision{newName: strings.TrimSpace(entry.Text)})
	}})

	fyne.Do(func() {
		showDialog(dialogConfig{
			Parent:    p.window,
			Title:     "AUTO-RENAME FILE?",
			Body:      body,
			Actions:   actions,
			WidthFrac: 0.6,
			AnchorTop: true,
		})
	})

	return <-ch
}

func (p *Panel) autoRenameAfterTranscribe(cacheKey, jsonPath, sourcePath, sourceName string, fallback time.Time) (string, string) {
	if sourcePath == "" {
		return sourcePath, sourceName
	}
	text, err := loadTranscriptText(jsonPath)
	if err != nil {
		p.AppendLog(fmt.Sprintf("  Auto-name skipped: %v", err))
		return sourcePath, sourceName
	}
	if strings.TrimSpace(text) == "" {
		p.AppendLog("  Auto-name skipped: transcript is empty")
		return sourcePath, sourceName
	}
	if fallback.IsZero() {
		if st, err := os.Stat(sourcePath); err == nil {
			fallback = st.ModTime()
		} else {
			fallback = time.Now()
		}
	}

	p.AppendLog("  Auto-naming...")
	p.setStatus("Auto-naming...")
	platsvc.UpdateProgress(-1, "Auto-naming…")
	renameStart := time.Now()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	s, err := namer.Suggest(ctx, text, fallback)
	if err != nil {
		if errors.Is(err, llm.ErrNoLLMInstalled) {
			p.AppendLog("  Auto-name skipped: no LLM installed (download one in Settings → Models)")
			return sourcePath, sourceName
		}
		p.AppendLog(fmt.Sprintf("  Auto-name failed after %.0fs: %v", time.Since(renameStart).Seconds(), err))
		return sourcePath, sourceName
	}
	p.AppendLog(fmt.Sprintf("  Auto-name suggested in %.0fs", time.Since(renameStart).Seconds()))

	ext := filepath.Ext(sourcePath)
	suggested := s.Filename(ext)

	regenerate := func() (string, error) {
		rctx, rcancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer rcancel()
		sRe, rerr := namer.Suggest(rctx, text, fallback)
		if rerr != nil {
			return "", rerr
		}
		return sRe.Filename(ext), nil
	}
	decision := p.promptRename(sourceName, suggested, regenerate)
	if decision.keep {
		p.AppendLog("  Auto-name: kept original name")
		return sourcePath, sourceName
	}
	finalName := decision.newName
	if finalName == "" {
		p.AppendLog("  Auto-name: empty name, kept original")
		return sourcePath, sourceName
	}

	if ext != "" && filepath.Ext(finalName) == "" {
		finalName += ext
	}
	dst := filepath.Join(filepath.Dir(sourcePath), finalName)
	if dst == sourcePath {
		p.AppendLog("  Auto-name: already named: " + finalName)
		return sourcePath, sourceName
	}
	if _, err := os.Stat(dst); err == nil {
		p.AppendLog("  Auto-name skipped: destination exists: " + finalName)
		return sourcePath, sourceName
	}
	if err := os.Rename(sourcePath, dst); err != nil {
		p.AppendLog(fmt.Sprintf("  Auto-name failed: rename: %v", err))
		return sourcePath, sourceName
	}
	if cacheKey != "" {
		if err := cache.SetSource(cacheKey, dst, finalName); err != nil {
			p.AppendLog(fmt.Sprintf("  Auto-name: cache update failed: %v", err))
		}
	}
	p.AppendLog("  Renamed: " + finalName)
	return dst, finalName
}

func loadTranscriptText(jsonPath string) (string, error) {
	tr, err := loadTranscript(jsonPath)
	if err != nil {
		return "", err
	}
	return transcriptToText(tr), nil
}

func transcriptToText(tr *transcriber.Transcript) string {
	var sb strings.Builder
	for _, u := range tr.Utterances {
		if u.Speaker != "" {
			sb.WriteString(u.Speaker)
			sb.WriteString(": ")
		}
		sb.WriteString(strings.TrimSpace(u.Text))
		sb.WriteByte('\n')
	}
	return sb.String()
}
