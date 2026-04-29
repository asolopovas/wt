package gui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"

	shared "github.com/asolopovas/wt/internal"
	"github.com/asolopovas/wt/internal/progress"
)

var rtfMu sync.Mutex

func rtfPath() string {
	return filepath.Join(shared.Dir(), "rtf.json")
}

func rtfKey(model, device string) string {
	return strings.ToLower(model) + "|" + strings.ToLower(device)
}

func loadRTF(model, device string) float64 {
	rtfMu.Lock()
	defer rtfMu.Unlock()
	data, err := os.ReadFile(rtfPath())
	if err != nil {
		return progress.DefaultRTF(model, device)
	}
	var m map[string]float64
	if err := json.Unmarshal(data, &m); err != nil {
		return progress.DefaultRTF(model, device)
	}
	if v, ok := m[rtfKey(model, device)]; ok && v > 0 {
		return v
	}
	return progress.DefaultRTF(model, device)
}

func saveRTF(model, device string, observed float64) {
	if observed <= 0 {
		return
	}
	rtfMu.Lock()
	defer rtfMu.Unlock()
	m := map[string]float64{}
	if data, err := os.ReadFile(rtfPath()); err == nil {
		_ = json.Unmarshal(data, &m)
	}
	key := rtfKey(model, device)
	if prev, ok := m[key]; ok && prev > 0 {
		m[key] = 0.5*prev + 0.5*observed
	} else {
		m[key] = observed
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return
	}
	if err := os.MkdirAll(shared.Dir(), 0o755); err != nil {
		return
	}
	_ = os.WriteFile(rtfPath(), data, 0o644)
}
