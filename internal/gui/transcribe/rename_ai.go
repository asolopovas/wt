package transcribe

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/asolopovas/wt/internal/namer"
	"github.com/asolopovas/wt/internal/transcriber"
)

func (p *Panel) AISuggestRename(item ExportItem, fallback time.Time) {
	tr, err := loadTranscript(item.CachePath)
	if err != nil {
		showError(p.window, fmt.Errorf("loading %s: %w", item.SourceName, err))
		return
	}
	text := transcriptToText(tr)
	if strings.TrimSpace(text) == "" {
		showNotice(p.window, notifyInfo, "Auto-name", "Transcript is empty.")
		return
	}

	if fallback.IsZero() {
		fallback = time.Now()
	}

	statusLbl := widget.NewLabel("Generating with active LLM…")
	body := container.NewVBox(statusLbl, newThinProgress())
	hide := showDialog(dialogConfig{
		Parent: p.window,
		Title:  "AUTO-NAME",
		Body:   body,
		Actions: []dialogAction{
			{Label: "CANCEL", Kind: kindSecondary},
		},
		WidthFrac: 0.5,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)

	go func() {
		defer cancel()
		s, err := namer.Suggest(ctx, text, fallback)
		fyne.Do(func() {
			hide()
			if err != nil {
				showError(p.window, err)
				return
			}
			p.showRenameSuggestion(item, s)
		})
	}()
}

func (p *Panel) showRenameSuggestion(item ExportItem, s namer.Suggestion) {
	srcExt := filepath.Ext(item.SourcePath)
	if srcExt == "" {
		srcExt = filepath.Ext(item.SourceName)
	}
	suggested := s.Filename(srcExt)

	stamp := canvas.NewText(suggested, colPrimary)
	stamp.TextSize = textBody
	stamp.TextStyle = monoBoldStyle

	hint := canvas.NewText("Suggested filename", colMuted)
	hint.TextSize = textCaption

	body := container.NewVBox(hint, stamp)

	canRename := item.SourcePath != ""
	if canRename {
		if _, err := os.Stat(item.SourcePath); err != nil {
			canRename = false
		}
	}

	actions := []dialogAction{
		{Label: "CLOSE", Kind: kindSecondary},
		{Label: "COPY", Kind: kindSecondary, OnTap: func() {
			fyne.CurrentApp().Clipboard().SetContent(suggested)
			showNotice(p.window, notifyInfo, "Auto-name", "Filename copied.")
		}},
	}
	if canRename {
		actions = append(actions, dialogAction{
			Label: "RENAME FILE", Kind: kindPrimary, OnTap: func() {
				dst := filepath.Join(filepath.Dir(item.SourcePath), suggested)
				if dst == item.SourcePath {
					showNotice(p.window, notifyInfo, "Auto-name", "Already named.")
					return
				}
				if _, err := os.Stat(dst); err == nil {
					showError(p.window, fmt.Errorf("destination exists: %s", dst))
					return
				}
				if err := os.Rename(item.SourcePath, dst); err != nil {
					showError(p.window, fmt.Errorf("rename: %w", err))
					return
				}
				showNotice(p.window, notifyInfo, "Auto-name", "File renamed.")
			},
		})
	}

	showDialog(dialogConfig{
		Parent:    p.window,
		Title:     "AUTO-NAME",
		Body:      body,
		Actions:   actions,
		WidthFrac: 0.5,
	})
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
