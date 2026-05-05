package models

import (
	"path/filepath"
	"runtime"
	"sync"

	shared "github.com/asolopovas/wt/internal"
)

type Family string

const (
	FamilyDiarizer Family = "diarizer"
	FamilyLLM      Family = "llm"
	FamilyASR      Family = "asr"
)

type Entry = shared.Model
type FileSpec = shared.ModelFile

var (
	mu      sync.RWMutex
	entries []Entry
)

func Set(in []Entry) {
	mu.Lock()
	entries = make([]Entry, len(in))
	copy(entries, in)
	mu.Unlock()
}

func loadCatalog() []Entry {
	mu.RLock()
	if entries != nil {
		out := make([]Entry, len(entries))
		copy(out, entries)
		mu.RUnlock()
		return out
	}
	mu.RUnlock()

	cfg, err := shared.Load()
	if err == nil && len(cfg.Models) > 0 {
		Set(cfg.Models)
		mu.RLock()
		out := make([]Entry, len(entries))
		copy(out, entries)
		mu.RUnlock()
		return out
	}
	def := shared.Defaults()
	Set(def.Models)
	mu.RLock()
	out := make([]Entry, len(entries))
	copy(out, entries)
	mu.RUnlock()
	return out
}

func LanguagesFor(id string) []string {
	e, ok := ByID(id)
	if !ok {
		return nil
	}
	return e.Languages
}

func Catalog() []Entry {
	return loadCatalog()
}

func ByID(id string) (Entry, bool) {
	for _, e := range loadCatalog() {
		if e.ID == id {
			return e, true
		}
	}
	return Entry{}, false
}

func ByFamily(f Family) []Entry {
	out := []Entry{}
	for _, e := range loadCatalog() {
		if Family(e.Family) == f {
			out = append(out, e)
		}
	}
	return out
}

func EngineForActiveASR(activeASR string) (engine, modelID string) {
	if activeASR != "" {
		if e, ok := ByID(activeASR); ok && Family(e.Family) == FamilyASR && e.Engine != "" {
			return e.Engine, e.ID
		}
	}
	return shared.EngineWhisper, ""
}

func DefaultID(f Family) string {
	list := ByFamily(f)
	if runtime.GOOS == "android" {
		for _, e := range list {
			if e.AndroidDefault {
				return e.ID
			}
		}
	}
	for _, e := range list {
		if e.DefaultActive {
			return e.ID
		}
	}
	if len(list) > 0 {
		return list[0].ID
	}
	return ""
}

func DirFor(e Entry) string {
	if len(e.Files) == 0 {
		return ""
	}
	return filepath.Join(shared.ModelsDir(), filepath.Dir(e.Files[0].RelPath))
}

func DirForID(id string) string {
	if e, ok := ByID(id); ok {
		return DirFor(e)
	}
	return ""
}
