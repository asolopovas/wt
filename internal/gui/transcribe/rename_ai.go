package transcribe

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/asolopovas/wt/internal/gui/platsvc"
	"github.com/asolopovas/wt/internal/llm"
	"github.com/asolopovas/wt/internal/namer"
	"github.com/asolopovas/wt/internal/transcriber"
	"github.com/asolopovas/wt/internal/transcriber/cache"
)

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
	// Outer ctx is generous; the actual per-invocation timeout lives in
	// llm.runOnce so the error reports llama-cli stderr cleanly.
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
	dst := filepath.Join(filepath.Dir(sourcePath), suggested)
	if dst == sourcePath {
		p.AppendLog("  Auto-name: already named: " + suggested)
		return sourcePath, sourceName
	}
	if _, err := os.Stat(dst); err == nil {
		p.AppendLog("  Auto-name skipped: destination exists: " + suggested)
		return sourcePath, sourceName
	}
	if err := os.Rename(sourcePath, dst); err != nil {
		p.AppendLog(fmt.Sprintf("  Auto-name failed: rename: %v", err))
		return sourcePath, sourceName
	}
	if cacheKey != "" {
		if err := cache.SetSource(cacheKey, dst, suggested); err != nil {
			p.AppendLog(fmt.Sprintf("  Auto-name: cache update failed: %v", err))
		}
	}
	p.AppendLog("  Renamed: " + suggested)
	return dst, suggested
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
