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
	"github.com/asolopovas/wt/internal/diarizer"
)

func rawTranscriptDir() string {
	return filepath.Join(shared.CacheDir(), "raw")
}

func computeRawKey(sourcePath string, mtimeNs int64, model, language string) string {
	s := fmt.Sprintf("%s\x00%d\x00%s\x00%s", sourcePath, mtimeNs, model, language)
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])[:32]
}

func rawTranscriptPath(key string) string {
	return filepath.Join(rawTranscriptDir(), key+".json")
}

func loadRawSegments(key string) ([]diarizer.TranscriptSegment, bool) {
	data, err := os.ReadFile(rawTranscriptPath(key))
	if err != nil {
		return nil, false
	}
	var segs []diarizer.TranscriptSegment
	if err := json.Unmarshal(data, &segs); err != nil {
		return nil, false
	}
	return segs, true
}

func saveRawSegments(key string, segs []diarizer.TranscriptSegment) error {
	if err := os.MkdirAll(rawTranscriptDir(), 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(segs)
	if err != nil {
		return err
	}
	return os.WriteFile(rawTranscriptPath(key), data, 0o644)
}

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
	RecordedAt time.Time `json:"recorded_at,omitempty"`
	SizeBytes  int64     `json:"size_bytes"`
	Pending    bool      `json:"pending,omitempty"`
}

func recordedAtOrFallback(e cacheEntry) time.Time {
	if !e.RecordedAt.IsZero() {
		return e.RecordedAt
	}
	if e.SourcePath != "" {
		if info, err := os.Stat(e.SourcePath); err == nil {
			return info.ModTime().Local()
		}
	}
	return e.CreatedAt
}

func cacheSetRecordedAt(key string, t time.Time) error {
	entries, err := loadManifest()
	if err != nil {
		return err
	}
	for i := range entries {
		if entries[i].Key == key {
			entries[i].RecordedAt = t
			return saveManifest(entries)
		}
	}
	return fmt.Errorf("entry %s not found", key)
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
		if e.Key == entry.Key {
			if entry.RecordedAt.IsZero() && !e.RecordedAt.IsZero() {
				entry.RecordedAt = e.RecordedAt
			}
			continue
		}
		if e.Pending && e.SourcePath == entry.SourcePath {
			if entry.RecordedAt.IsZero() && !e.RecordedAt.IsZero() {
				entry.RecordedAt = e.RecordedAt
			}
			continue
		}
		filtered = append(filtered, e)
	}
	if entry.RecordedAt.IsZero() && entry.SourcePath != "" {
		if info, err := os.Stat(entry.SourcePath); err == nil {
			entry.RecordedAt = info.ModTime().Local()
		}
	}
	filtered = append(filtered, entry)
	if err := saveManifest(filtered); err != nil {
		return dst, err
	}
	return dst, nil
}

func pendingCacheKey(absPath string) string {
	sum := sha256.Sum256([]byte(absPath + "\x00pending"))
	return hex.EncodeToString(sum[:])[:32]
}

// cacheStorePending records sourcePath as a not-yet-transcribed entry in the manifest
// so it appears in the history list marked as fresh. Idempotent — re-adding the same
// path keeps the original CreatedAt.
func cacheStorePending(sourcePath string) error {
	abs, err := filepath.Abs(sourcePath)
	if err != nil {
		return err
	}
	info, statErr := os.Stat(abs)
	var size int64
	var recordedAt time.Time
	if statErr == nil {
		size = info.Size()
		recordedAt = info.ModTime().Local()
	}

	key := pendingCacheKey(abs)
	entries, _ := loadManifest()
	for _, e := range entries {
		if e.Key == key {
			return nil
		}
	}
	entries = append(entries, cacheEntry{
		Key:        key,
		SourcePath: abs,
		SourceName: filepath.Base(abs),
		CreatedAt:  time.Now(),
		RecordedAt: recordedAt,
		SizeBytes:  size,
		Pending:    true,
	})
	return saveManifest(entries)
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
