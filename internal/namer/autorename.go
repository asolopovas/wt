package namer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type RenameResult struct {
	AudioPath string
	JSONPath  string
	Topic     string
	Stamp     string
}

func AutoRename(ctx context.Context, audioPath, jsonPath string, fallback time.Time) (RenameResult, error) {
	res := RenameResult{AudioPath: audioPath, JSONPath: jsonPath}

	text, err := ExtractTranscriptText(jsonPath)
	if err != nil {
		return res, fmt.Errorf("reading transcript: %w", err)
	}
	if strings.TrimSpace(text) == "" {
		return res, fmt.Errorf("transcript is empty")
	}

	if fallback.IsZero() {
		if st, err := os.Stat(audioPath); err == nil {
			fallback = st.ModTime()
		} else {
			fallback = time.Now()
		}
	}

	s, err := Suggest(ctx, text, fallback)
	if err != nil {
		return res, err
	}
	res.Topic = s.Topic
	res.Stamp = s.Stamp

	if audioPath != "" {
		if newPath, err := renameWithSlug(audioPath, s, ""); err != nil {
			return res, fmt.Errorf("rename audio: %w", err)
		} else {
			res.AudioPath = newPath
		}
	}
	if jsonPath != "" {
		if newPath, err := renameWithSlug(jsonPath, s, ".json"); err != nil {
			return res, fmt.Errorf("rename transcript: %w", err)
		} else {
			res.JSONPath = newPath
		}
	}
	return res, nil
}

func renameWithSlug(path string, s Suggestion, forceExt string) (string, error) {
	ext := forceExt
	if ext == "" {
		ext = filepath.Ext(path)
	}
	dst := filepath.Join(filepath.Dir(path), s.Filename(ext))
	if dst == path {
		return path, nil
	}
	if _, err := os.Stat(dst); err == nil {
		return path, fmt.Errorf("destination exists: %s", filepath.Base(dst))
	}
	if err := os.Rename(path, dst); err != nil {
		return path, err
	}
	return dst, nil
}
