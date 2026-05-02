package cache

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/asolopovas/wt/internal/diarizer"
)

func redirectAppDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	if err := os.MkdirAll(filepath.Join(dir, "wt"), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestComputeRawKey_StableAndDistinguishing(t *testing.T) {
	a := ComputeRawKey("/x/y", 1, "tiny", "en")
	b := ComputeRawKey("/x/y", 1, "tiny", "en")
	if a != b {
		t.Errorf("not deterministic: %s vs %s", a, b)
	}
	if len(a) != 32 {
		t.Errorf("len=%d want 32", len(a))
	}
	if ComputeRawKey("/x/y", 2, "tiny", "en") == a {
		t.Error("mtime change should alter key")
	}
	if ComputeRawKey("/x/y", 1, "small", "en") == a {
		t.Error("model change should alter key")
	}
	if ComputeRawKey("/x/y", 1, "tiny", "fr") == a {
		t.Error("language change should alter key")
	}
}

func TestComputeCacheKey_HonorsAllFields(t *testing.T) {
	base := KeyParams{SourcePath: "/a", MtimeNs: 1, Model: "tiny", Language: "en", Speakers: 0, NoDiarize: false}
	k := ComputeKey(base)

	mod := base
	mod.Speakers = 2
	if ComputeKey(mod) == k {
		t.Error("speakers should affect key")
	}
	mod = base
	mod.NoDiarize = true
	if ComputeKey(mod) == k {
		t.Error("noDiarize should affect key")
	}
}

func TestRawCacheSafe(t *testing.T) {
	tests := []struct {
		name      string
		segs      []diarizer.TranscriptSegment
		dur       float64
		cancelled bool
		safe      bool
	}{
		{"cancelled", []diarizer.TranscriptSegment{{End: time.Second}}, 1, true, false},
		{"empty", nil, 1, false, false},
		{"unknown duration ok", []diarizer.TranscriptSegment{{End: time.Second}}, 0, false, true},
		{"low coverage", []diarizer.TranscriptSegment{{End: 10 * time.Second}}, 100, false, false},
		{"good coverage", []diarizer.TranscriptSegment{{End: 80 * time.Second}}, 100, false, true},
	}
	for _, tt := range tests {
		ok, _ := RawCacheSafe(tt.segs, tt.dur, tt.cancelled)
		if ok != tt.safe {
			t.Errorf("%s: safe=%v want %v", tt.name, ok, tt.safe)
		}
	}
}

func TestPendingCacheKey_StableAndDistinct(t *testing.T) {
	a1 := pendingCacheKey("/a")
	a2 := pendingCacheKey("/a")
	b := pendingCacheKey("/b")
	if a1 != a2 {
		t.Error("not deterministic")
	}
	if a1 == b {
		t.Error("expected different keys")
	}
}

func TestRawSegmentsRoundTrip(t *testing.T) {
	redirectAppDir(t)
	segs := []diarizer.TranscriptSegment{
		{Start: 0, End: time.Second, Text: "hi"},
		{Start: time.Second, End: 2 * time.Second, Text: "yo"},
	}
	if err := SaveRawSegments("k1", segs); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, ok := LoadRawSegments("k1")
	if !ok {
		t.Fatal("LoadRawSegments not ok")
	}
	if len(got) != 2 || got[0].Text != "hi" || got[1].Text != "yo" {
		t.Errorf("round-trip mismatch: %+v", got)
	}
	if _, ok := LoadRawSegments("missing"); ok {
		t.Error("missing key should not load")
	}
}

func TestCacheStoreLookupAndDelete(t *testing.T) {
	redirectAppDir(t)
	entry := Entry{
		Key:        "abc",
		SourcePath: "/some/file",
		SourceName: "file.m4a",
		Model:      "tiny",
		Language:   "en",
		Utterances: 5,
		CreatedAt:  time.Now(),
	}
	dst, err := Store(entry, []byte(`{"language":"en"}`))
	if err != nil {
		t.Fatalf("Store: %v", err)
	}
	if _, statErr := os.Stat(dst); statErr != nil {
		t.Fatalf("transcript file missing: %v", statErr)
	}
	path, e, ok := Lookup("abc")
	if !ok || e == nil || e.Key != "abc" || path != dst {
		t.Errorf("lookup mismatch: ok=%v e=%+v path=%q", ok, e, path)
	}
	if err := Delete("abc"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, _, ok := Lookup("abc"); ok {
		t.Error("lookup should fail after delete")
	}
}

func TestCacheStore_ReplacesPendingForSamePath(t *testing.T) {
	redirectAppDir(t)
	srcPath := filepath.Join(t.TempDir(), "audio.m4a")
	if err := os.WriteFile(srcPath, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := StorePending(srcPath); err != nil {
		t.Fatalf("pending: %v", err)
	}
	entries, _ := loadManifest()
	if len(entries) != 1 || !entries[0].Pending {
		t.Fatalf("expected 1 pending entry, got %+v", entries)
	}

	abs, _ := filepath.Abs(srcPath)
	entry := Entry{Key: "real", SourcePath: abs, SourceName: "audio.m4a", CreatedAt: time.Now()}
	if _, err := Store(entry, []byte("{}")); err != nil {
		t.Fatalf("store: %v", err)
	}
	entries, _ = loadManifest()
	if len(entries) != 1 || entries[0].Pending {
		t.Fatalf("pending entry should have been replaced; got %+v", entries)
	}
}

func TestCacheGC_RemovesExpired(t *testing.T) {
	redirectAppDir(t)
	old := Entry{Key: "old", SourcePath: "/o", CreatedAt: time.Now().Add(-48 * time.Hour)}
	fresh := Entry{Key: "new", SourcePath: "/n", CreatedAt: time.Now()}
	if _, err := Store(old, []byte("{}")); err != nil {
		t.Fatal(err)
	}
	if _, err := Store(fresh, []byte("{}")); err != nil {
		t.Fatal(err)
	}
	removed := GC(1)
	if removed != 1 {
		t.Errorf("removed=%d want 1", removed)
	}
	if _, _, ok := Lookup("old"); ok {
		t.Error("old should be gone")
	}
	if _, _, ok := Lookup("new"); !ok {
		t.Error("new should remain")
	}
	if GC(0) != 0 {
		t.Error("expiryDays=0 should be a no-op")
	}
}

func TestCacheEntriesByRecent_DedupesPrefersReal(t *testing.T) {
	redirectAppDir(t)
	now := time.Now()
	entries := []Entry{
		{Key: "p", SourcePath: "/x", CreatedAt: now.Add(-time.Hour), Pending: true},
		{Key: "r", SourcePath: "/x", CreatedAt: now.Add(-2 * time.Hour)},
	}
	if err := saveManifest(entries); err != nil {
		t.Fatal(err)
	}
	got := EntriesByRecent()
	if len(got) != 1 {
		t.Fatalf("len=%d want 1", len(got))
	}
	if got[0].Pending {
		t.Error("real entry should win over pending")
	}
}

func TestRecordedAtOrFallback(t *testing.T) {
	explicit := time.Now().Add(-3 * time.Hour)
	if got := RecordedAtOrFallback(Entry{RecordedAt: explicit}); !got.Equal(explicit) {
		t.Errorf("explicit RecordedAt should win, got %v", got)
	}

	tmp := filepath.Join(t.TempDir(), "audio.m4a")
	if err := os.WriteFile(tmp, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(tmp)
	got := RecordedAtOrFallback(Entry{SourcePath: tmp, CreatedAt: time.Now()})
	if !got.Equal(info.ModTime().Local()) {
		t.Errorf("source mtime fallback expected %v, got %v", info.ModTime().Local(), got)
	}

	created := time.Now().Add(-time.Hour)
	got = RecordedAtOrFallback(Entry{SourcePath: "/nonexistent/file", CreatedAt: created})
	if !got.Equal(created) {
		t.Errorf("CreatedAt fallback expected %v, got %v", created, got)
	}

	got = RecordedAtOrFallback(Entry{CreatedAt: created})
	if !got.Equal(created) {
		t.Errorf("empty SourcePath: CreatedAt expected %v, got %v", created, got)
	}
}

func TestSetRecordedAt(t *testing.T) {
	redirectAppDir(t)
	if _, err := Store(Entry{Key: "k1", CreatedAt: time.Now()}, []byte("{}")); err != nil {
		t.Fatal(err)
	}

	when := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	if err := SetRecordedAt("k1", when); err != nil {
		t.Fatalf("SetRecordedAt: %v", err)
	}

	entries, _ := loadManifest()
	if len(entries) != 1 || !entries[0].RecordedAt.Equal(when) {
		t.Errorf("RecordedAt not persisted, got %+v", entries)
	}

	if err := SetRecordedAt("missing", when); err == nil {
		t.Error("expected error for missing key")
	}
}

func TestBuildKeyParams(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "a.wav")
	if err := os.WriteFile(tmp, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	p, err := BuildKeyParams(tmp, "small", "en", 2, true)
	if err != nil {
		t.Fatalf("BuildKeyParams: %v", err)
	}
	if !filepath.IsAbs(p.SourcePath) {
		t.Errorf("SourcePath should be absolute, got %q", p.SourcePath)
	}
	if p.MtimeNs == 0 {
		t.Error("MtimeNs should be set")
	}
	if p.Model != "small" || p.Language != "en" || p.Speakers != 2 || !p.NoDiarize {
		t.Errorf("fields not propagated: %+v", p)
	}

	if _, err := BuildKeyParams("/nonexistent/audio.wav", "tiny", "en", 0, false); err == nil {
		t.Error("expected error for missing file")
	}
}

func TestClear(t *testing.T) {
	redirectAppDir(t)
	if _, err := Store(Entry{Key: "a", CreatedAt: time.Now()}, []byte("{}")); err != nil {
		t.Fatal(err)
	}
	if _, err := Store(Entry{Key: "b", CreatedAt: time.Now()}, []byte("{}")); err != nil {
		t.Fatal(err)
	}

	if err := Clear(); err != nil {
		t.Fatalf("Clear: %v", err)
	}

	if _, err := os.Stat(TranscriptPathForKey("a")); !os.IsNotExist(err) {
		t.Errorf("transcript a should be gone, stat err=%v", err)
	}
	if _, err := os.Stat(TranscriptPathForKey("b")); !os.IsNotExist(err) {
		t.Errorf("transcript b should be gone, stat err=%v", err)
	}

	redirectAppDir(t)
	if err := Clear(); err != nil {
		t.Errorf("Clear on missing dir should be a no-op, got %v", err)
	}
}

func TestSpeakerRenamesRoundTrip(t *testing.T) {
	redirectAppDir(t)
	if err := SaveSpeakerRenames("k", map[string]string{"S1": "Alice"}); err != nil {
		t.Fatal(err)
	}
	got := LoadSpeakerRenames("k")
	if got["S1"] != "Alice" {
		t.Errorf("got=%v", got)
	}
	if err := SaveSpeakerRenames("k", map[string]string{}); err != nil {
		t.Fatal(err)
	}
	if got := LoadSpeakerRenames("k"); got != nil {
		t.Errorf("empty map should remove file; loaded=%v", got)
	}
}
