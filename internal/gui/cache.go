package gui

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	shared "github.com/asolopovas/wt/internal"
)

type cacheEntry struct {
	Key        string    `json:"key"`
	SourcePath string    `json:"source_path"`
	SourceName string    `json:"source_name"`
	Model      string    `json:"model"`
	Language   string    `json:"language"`
	Speakers   int       `json:"speakers"`
	NoDiarize  bool      `json:"no_diarize"`
	Utterances int       `json:"utterances"`
	CreatedAt  time.Time `json:"created_at"`
	SizeBytes  int64     `json:"size_bytes"`
}

type cacheKeyParams struct {
	SourcePath string
	MtimeNs    int64
	Model      string
	Language   string
	Speakers   int
	NoDiarize  bool
}

func transcriptCacheDir() string {
	return filepath.Join(shared.CacheDir(), "transcripts")
}

func transcriptIndexPath() string {
	return filepath.Join(transcriptCacheDir(), "index.json")
}

func transcriptPathForKey(key string) string {
	return filepath.Join(transcriptCacheDir(), key+".json")
}

func computeCacheKey(p cacheKeyParams) string {
	s := fmt.Sprintf("%s\x00%d\x00%s\x00%s\x00%d\x00%v",
		p.SourcePath, p.MtimeNs, p.Model, p.Language, p.Speakers, p.NoDiarize)
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])[:32]
}

func buildCacheParams(sourcePath, model, language string, speakers int, noDiarize bool) (cacheKeyParams, error) {
	abs, err := filepath.Abs(sourcePath)
	if err != nil {
		return cacheKeyParams{}, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return cacheKeyParams{}, err
	}
	return cacheKeyParams{
		SourcePath: abs,
		MtimeNs:    info.ModTime().UnixNano(),
		Model:      model,
		Language:   language,
		Speakers:   speakers,
		NoDiarize:  noDiarize,
	}, nil
}

func loadManifest() ([]cacheEntry, error) {
	data, err := os.ReadFile(transcriptIndexPath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var entries []cacheEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}

func saveManifest(entries []cacheEntry) error {
	if err := os.MkdirAll(transcriptCacheDir(), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(transcriptIndexPath(), data, 0o644)
}

func cacheLookup(key string) (string, *cacheEntry, bool) {
	path := transcriptPathForKey(key)
	if _, err := os.Stat(path); err != nil {
		return "", nil, false
	}
	entries, _ := loadManifest()
	for i := range entries {
		if entries[i].Key == key {
			return path, &entries[i], true
		}
	}
	return path, nil, true
}

func cacheStore(entry cacheEntry, transcriptJSON []byte) (string, error) {
	if err := os.MkdirAll(transcriptCacheDir(), 0o755); err != nil {
		return "", err
	}
	dst := transcriptPathForKey(entry.Key)
	if err := os.WriteFile(dst, transcriptJSON, 0o644); err != nil {
		return "", err
	}
	entry.SizeBytes = int64(len(transcriptJSON))

	entries, _ := loadManifest()
	filtered := entries[:0]
	for _, e := range entries {
		if e.Key != entry.Key {
			filtered = append(filtered, e)
		}
	}
	filtered = append(filtered, entry)
	if err := saveManifest(filtered); err != nil {
		return dst, err
	}
	return dst, nil
}

func cacheGC(expiryDays int) int {
	if expiryDays <= 0 {
		return 0
	}
	entries, err := loadManifest()
	if err != nil || len(entries) == 0 {
		return 0
	}
	cutoff := time.Now().Add(-time.Duration(expiryDays) * 24 * time.Hour)
	kept := make([]cacheEntry, 0, len(entries))
	removed := 0
	for _, e := range entries {
		if e.CreatedAt.Before(cutoff) {
			_ = os.Remove(transcriptPathForKey(e.Key))
			removed++
			continue
		}
		kept = append(kept, e)
	}
	if removed > 0 {
		_ = saveManifest(kept)
	}
	return removed
}

func cacheDelete(key string) error {
	entries, _ := loadManifest()
	kept := entries[:0]
	for _, e := range entries {
		if e.Key == key {
			_ = os.Remove(transcriptPathForKey(e.Key))
			continue
		}
		kept = append(kept, e)
	}
	return saveManifest(kept)
}

func cacheClear() error {
	dir := transcriptCacheDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, e := range entries {
		_ = os.Remove(filepath.Join(dir, e.Name()))
	}
	return nil
}

func cacheEntriesByRecent() []cacheEntry {
	entries, _ := loadManifest()
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].CreatedAt.After(entries[j].CreatedAt)
	})
	return entries
}
