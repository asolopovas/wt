package shared

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaults(t *testing.T) {
	cfg := Defaults()
	if cfg.Device != "auto" {
		t.Errorf("expected device 'auto', got %q", cfg.Device)
	}
	if cfg.Threads <= 0 {
		t.Errorf("expected positive thread count, got %d", cfg.Threads)
	}
	if len(cfg.Models) == 0 {
		t.Errorf("expected default Models populated, got empty")
	}
}

func TestDir(t *testing.T) {
	dir := Dir()
	if dir == "" {
		t.Fatal("Dir returned empty string")
	}
	if filepath.Base(dir) != "wt" {
		t.Errorf("expected dir to end with 'wt', got %q", dir)
	}
}

func TestModelsDir(t *testing.T) {
	dir := ModelsDir()
	if filepath.Base(dir) != "models" {
		t.Errorf("expected models dir to end with 'models', got %q", dir)
	}
}

func TestCacheDir(t *testing.T) {
	dir := CacheDir()
	if filepath.Base(dir) != "cache" {
		t.Errorf("expected cache dir to end with 'cache', got %q", dir)
	}
}

func TestFilePath(t *testing.T) {
	fp := FilePath()
	if filepath.Base(fp) != "config.yml" {
		t.Errorf("expected file path to end with 'config.yml', got %q", fp)
	}
}

func TestApplyEnvOverrides(t *testing.T) {
	t.Setenv("WT_MODEL", "small")
	t.Setenv("WT_LANGUAGE", "en")
	t.Setenv("WT_DEVICE", "cpu")
	t.Setenv("WT_THREADS", "8")
	t.Setenv("WT_SPEAKERS", "3")
	t.Setenv("WT_NO_DIARIZE", "true")
	t.Setenv("WT_CACHE_EXPIRY_DAYS", "7")

	cfg := Defaults()
	applyEnvOverrides(&cfg)

	if cfg.Model != "small" {
		t.Errorf("Model: got %q, want %q", cfg.Model, "small")
	}
	if cfg.Language != "en" {
		t.Errorf("Language: got %q, want %q", cfg.Language, "en")
	}
	if cfg.Device != "cpu" {
		t.Errorf("Device: got %q, want %q", cfg.Device, "cpu")
	}
	if cfg.Threads != 8 {
		t.Errorf("Threads: got %d, want 8", cfg.Threads)
	}
	if cfg.Speakers != 3 {
		t.Errorf("Speakers: got %d, want 3", cfg.Speakers)
	}
	if !cfg.NoDiarize {
		t.Errorf("NoDiarize: got false, want true")
	}
	if cfg.CacheExpiryDays != 7 {
		t.Errorf("CacheExpiryDays: got %d, want 7", cfg.CacheExpiryDays)
	}
}

func TestApplyEnvOverrides_IgnoresInvalid(t *testing.T) {
	t.Setenv("WT_THREADS", "not-a-number")
	t.Setenv("WT_SPEAKERS", "-1")
	t.Setenv("WT_NO_DIARIZE", "maybe")

	cfg := Config{Threads: 4, Speakers: 2, NoDiarize: false}
	applyEnvOverrides(&cfg)

	if cfg.Threads != 4 {
		t.Errorf("Threads: got %d, want 4 (invalid env should be ignored)", cfg.Threads)
	}
	if cfg.Speakers != 2 {
		t.Errorf("Speakers: got %d, want 2 (negative env should be ignored)", cfg.Speakers)
	}
	if cfg.NoDiarize {
		t.Errorf("NoDiarize: got true, want false (invalid bool should be ignored)")
	}
}

func TestSaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)
	t.Setenv("APPDATA", tmpDir)
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	cfg := Config{
		Model:   "sherpa-whisper-base.en",
		Device:  "cpu",
		Threads: 4,
	}

	if err := os.MkdirAll(filepath.Dir(FilePath()), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := Save(cfg); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if loaded.Model != cfg.Model {
		t.Errorf("Model: got %q, want %q", loaded.Model, cfg.Model)
	}
	if loaded.Device != cfg.Device {
		t.Errorf("Device: got %q, want %q", loaded.Device, cfg.Device)
	}
	if loaded.Threads != cfg.Threads {
		t.Errorf("Threads: got %d, want %d", loaded.Threads, cfg.Threads)
	}
}
