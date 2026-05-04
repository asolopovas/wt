package cache

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

var ProbeDurationMsFn = func(string) int64 { return 0 }

func rawTranscriptDir() string {
	return filepath.Join(shared.CacheDir(), "raw")
}

func ComputeRawKey(sourcePath string, mtimeNs int64, model, language string) string {
	s := fmt.Sprintf("%s\x00%d\x00%s\x00%s", sourcePath, mtimeNs, model, language)
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])[:32]
}

func rawTranscriptPath(key string) string {
	return filepath.Join(rawTranscriptDir(), key+".json")
}

func LoadRawSegments(key string) ([]diarizer.TranscriptSegment, bool) {
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

func RawCacheSafe(segs []diarizer.TranscriptSegment, audioDurSec float64, cancelled bool) (bool, string) {
	if cancelled {
		return false, "transcription cancelled"
	}
	if len(segs) == 0 {
		return false, "no segments produced"
	}
	if audioDurSec <= 0 {
		return true, ""
	}
	lastEnd := time.Duration(0)
	for _, s := range segs {
		if s.End > lastEnd {
			lastEnd = s.End
		}
	}
	coverage := lastEnd.Seconds() / audioDurSec
	if coverage < 0.5 {
		return false, fmt.Sprintf("coverage %.0f%% < 50%% (last_end=%.1fs, dur=%.1fs)", coverage*100, lastEnd.Seconds(), audioDurSec)
	}
	return true, ""
}

func SaveRawSegments(key string, segs []diarizer.TranscriptSegment) error {
	if err := os.MkdirAll(rawTranscriptDir(), 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(segs)
	if err != nil {
		return err
	}
	return os.WriteFile(rawTranscriptPath(key), data, 0o644)
}

type Partial struct {
	Segments   []diarizer.TranscriptSegment `json:"segments"`
	LastEndMs  int64                        `json:"last_end_ms"`
	AudioDurMs int64                        `json:"audio_dur_ms"`
	SavedAt    time.Time                    `json:"saved_at"`
}

func partialPath(key string) string {
	return filepath.Join(rawTranscriptDir(), key+".partial.json")
}

func LoadPartial(key string) (*Partial, bool) {
	data, err := os.ReadFile(partialPath(key))
	if err != nil {
		return nil, false
	}
	var p Partial
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, false
	}
	if p.LastEndMs <= 0 || len(p.Segments) == 0 {
		return nil, false
	}
	return &p, true
}

func SavePartial(key string, p Partial) error {
	if err := os.MkdirAll(rawTranscriptDir(), 0o755); err != nil {
		return err
	}
	if p.SavedAt.IsZero() {
		p.SavedAt = time.Now()
	}
	data, err := json.Marshal(p)
	if err != nil {
		return err
	}
	return os.WriteFile(partialPath(key), data, 0o644)
}

func DeletePartial(key string) {
	_ = os.Remove(partialPath(key))
}

func HasPartial(key string) bool {
	if key == "" {
		return false
	}
	_, err := os.Stat(partialPath(key))
	return err == nil
}

type Entry struct {
	Key        string    `json:"key"`
	SourcePath string    `json:"source_path"`
	SourceName string    `json:"source_name"`
	Model      string    `json:"model"`
	Language   string    `json:"language"`
	Speakers   int       `json:"speakers"`
	NoDiarize  bool      `json:"no_diarize"`
	Utterances int       `json:"utterances"`
	DurationMs int64     `json:"duration_ms,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	RecordedAt time.Time `json:"recorded_at,omitempty"`
	SizeBytes  int64     `json:"size_bytes"`
	Pending    bool      `json:"pending,omitempty"`
}

func RecordedAtOrFallback(e Entry) time.Time {
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

func SetRecordedAt(key string, t time.Time) error {
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

func SetSource(key, sourcePath, sourceName string) error {
	entries, err := loadManifest()
	if err != nil {
		return err
	}
	for i := range entries {
		if entries[i].Key == key {
			entries[i].SourcePath = sourcePath
			entries[i].SourceName = sourceName
			return saveManifest(entries)
		}
	}
	return fmt.Errorf("entry %s not found", key)
}

type KeyParams struct {
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

func TranscriptPathForKey(key string) string {
	return filepath.Join(transcriptCacheDir(), key+".json")
}

func ComputeKey(p KeyParams) string {
	s := fmt.Sprintf("%s\x00%d\x00%s\x00%s\x00%d\x00%v",
		p.SourcePath, p.MtimeNs, p.Model, p.Language, p.Speakers, p.NoDiarize)
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])[:32]
}

func BuildKeyParams(sourcePath, model, language string, speakers int, noDiarize bool) (KeyParams, error) {
	abs, err := filepath.Abs(sourcePath)
	if err != nil {
		return KeyParams{}, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return KeyParams{}, err
	}
	return KeyParams{
		SourcePath: abs,
		MtimeNs:    info.ModTime().UnixNano(),
		Model:      model,
		Language:   language,
		Speakers:   speakers,
		NoDiarize:  noDiarize,
	}, nil
}

func loadManifest() ([]Entry, error) {
	data, err := os.ReadFile(transcriptIndexPath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var entries []Entry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}

func saveManifest(entries []Entry) error {
	if err := os.MkdirAll(transcriptCacheDir(), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(transcriptIndexPath(), data, 0o644)
}

func InvalidateTranscript(key string) error {
	if err := os.Remove(TranscriptPathForKey(key)); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func Lookup(key string) (string, *Entry, bool) {
	path := TranscriptPathForKey(key)
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

func Export(key, dest string) error {
	src := TranscriptPathForKey(key)
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dest, data, 0o644)
}

func Store(entry Entry, transcriptJSON []byte) (string, error) {
	if err := os.MkdirAll(transcriptCacheDir(), 0o755); err != nil {
		return "", err
	}
	dst := TranscriptPathForKey(entry.Key)
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

func StorePending(sourcePath string) (string, error) {
	abs, err := filepath.Abs(sourcePath)
	if err != nil {
		return "", err
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
			return key, nil
		}
	}
	entries = append(entries, Entry{
		Key:        key,
		SourcePath: abs,
		SourceName: filepath.Base(abs),
		DurationMs: ProbeDurationMsFn(abs),
		CreatedAt:  time.Now(),
		RecordedAt: recordedAt,
		SizeBytes:  size,
		Pending:    true,
	})
	if err := saveManifest(entries); err != nil {
		return "", err
	}
	return key, nil
}

func BackfillDurations() int {
	entries, err := loadManifest()
	if err != nil || len(entries) == 0 {
		return 0
	}
	changed := 0
	for i := range entries {
		if entries[i].DurationMs > 0 || entries[i].SourcePath == "" {
			continue
		}
		if _, err := os.Stat(entries[i].SourcePath); err != nil {
			continue
		}
		if ms := ProbeDurationMsFn(entries[i].SourcePath); ms > 0 {
			entries[i].DurationMs = ms
			changed++
		}
	}
	if changed > 0 {
		_ = saveManifest(entries)
	}
	return changed
}

func GC(expiryDays int) int {
	if expiryDays <= 0 {
		return 0
	}
	entries, err := loadManifest()
	if err != nil || len(entries) == 0 {
		return 0
	}
	cutoff := time.Now().Add(-time.Duration(expiryDays) * 24 * time.Hour)
	kept := make([]Entry, 0, len(entries))
	removed := 0
	for _, e := range entries {
		if e.CreatedAt.Before(cutoff) {
			_ = os.Remove(TranscriptPathForKey(e.Key))
			_ = os.Remove(speakerRenamesPath(e.Key))
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

func Delete(key string) error {
	entries, _ := loadManifest()
	kept := entries[:0]
	for _, e := range entries {
		if e.Key == key {
			_ = os.Remove(TranscriptPathForKey(e.Key))
			_ = os.Remove(speakerRenamesPath(e.Key))
			continue
		}
		kept = append(kept, e)
	}
	return saveManifest(kept)
}

func Clear() error {
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

func speakerRenamesPath(key string) string {
	return filepath.Join(transcriptCacheDir(), key+"_speakers.json")
}

func LoadSpeakerRenames(key string) map[string]string {
	if key == "" {
		return nil
	}
	data, err := os.ReadFile(speakerRenamesPath(key))
	if err != nil {
		return nil
	}
	var m map[string]string
	if err := json.Unmarshal(data, &m); err != nil {
		return nil
	}
	return m
}

func SaveSpeakerRenames(key string, m map[string]string) error {
	if key == "" {
		return nil
	}
	path := speakerRenamesPath(key)
	if len(m) == 0 {
		_ = os.Remove(path)
		return nil
	}
	if err := os.MkdirAll(transcriptCacheDir(), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func EntriesByRecent() []Entry {
	entries, _ := loadManifest()
	kept := entries[:0]
	dropped := false
	for _, e := range entries {
		if e.SourcePath != "" {
			if _, err := os.Stat(e.SourcePath); err != nil && os.IsNotExist(err) {
				if e.Key != "" {
					_ = removeTranscriptForKey(e.Key)
				}
				dropped = true
				continue
			}
		}
		kept = append(kept, e)
	}
	if dropped {
		_ = saveManifest(kept)
	}
	entries = kept
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].CreatedAt.After(entries[j].CreatedAt)
	})

	seenPath := make(map[string]int, len(entries))
	deduped := make([]Entry, 0, len(entries))
	for _, e := range entries {
		path := e.SourcePath
		if path == "" {
			deduped = append(deduped, e)
			continue
		}
		if idx, ok := seenPath[path]; ok {
			if !deduped[idx].Pending && e.Pending {
				continue
			}
			if deduped[idx].Pending && !e.Pending {
				deduped[idx] = e
				continue
			}
			continue
		}
		seenPath[path] = len(deduped)
		deduped = append(deduped, e)
	}
	return deduped
}

func removeTranscriptForKey(key string) error {
	path := TranscriptPathForKey(key)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
