package main

import (
	"context"
	"os"
	"time"

	"github.com/asolopovas/wt/internal/namer"
	"github.com/asolopovas/wt/internal/ui"
)

func autoRename(audioPath, jsonPath string) {
	ui.Stage("Auto-naming...")

	fallback := time.Now()
	if st, err := os.Stat(audioPath); err == nil {
		fallback = st.ModTime()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	res, err := namer.AutoRename(ctx, audioPath, jsonPath, fallback)
	if err != nil {
		ui.Crossf("Auto-name skipped: %v", err)
		return
	}
	ui.Tickf("Renamed: %s", res.AudioPath)
	if res.JSONPath != jsonPath {
		ui.Tickf("Renamed: %s", res.JSONPath)
	}
}
