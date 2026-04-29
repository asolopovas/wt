package gui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"

	shared "github.com/asolopovas/wt/internal"
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
		return defaultRTF(model, device)
	}
	var m map[string]float64
	if err := json.Unmarshal(data, &m); err != nil {
		return defaultRTF(model, device)
	}
	if v, ok := m[rtfKey(model, device)]; ok && v > 0 {
		return v
	}
	return defaultRTF(model, device)
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
	_ = os.WriteFile(rtfPath(), data, 0o644)
}

func defaultRTF(model, device string) float64 {
	m := strings.ToLower(model)
	isCPU := device == "" || strings.Contains(strings.ToLower(device), "cpu")
	var base float64
	switch {
	case strings.HasPrefix(m, "tiny"):
		base = 30
	case strings.HasPrefix(m, "base"):
		base = 15
	case strings.HasPrefix(m, "small"):
		base = 8
	case strings.HasPrefix(m, "medium"):
		base = 4
	case strings.Contains(m, "large-v3-turbo") || strings.Contains(m, "turbo"):
		base = 6
	case strings.HasPrefix(m, "large"):
		base = 2
	default:
		base = 5
	}
	if isCPU {
		base /= 6
	}
	if base < 0.1 {
		base = 0.1
	}
	return base
}
