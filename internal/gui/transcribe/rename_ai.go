package transcribe

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/asolopovas/wt/internal/transcriber/cache"
	"github.com/asolopovas/wt/internal/namer"
	"github.com/asolopovas/wt/internal/transcriber"
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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	s, err := namer.Suggest(ctx, text, fallback)
	if err != nil {
		p.AppendLog(fmt.Sprintf("  Auto-name failed: %v", err))
		return sourcePath, sourceName
	}

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
