package shared

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestDefaults(t *testing.T) {
	cfg := Defaults()
	if cfg.Model != "turbo" {
		t.Errorf("expected model 'turbo', got %q", cfg.Model)
	}
	if cfg.Device != "auto" {
		t.Errorf("expected device 'auto', got %q", cfg.Device)
	}
	if cfg.Threads != runtime.NumCPU() {
		t.Errorf("expected %d threads, got %d", runtime.NumCPU(), cfg.Threads)
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

func TestSaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)
	t.Setenv("APPDATA", tmpDir)
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	cfg := Config{
		Model:   "small",
		Device:  "cpu",
		Threads: 4,
		TDRZ:    true,
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
	if loaded.TDRZ != cfg.TDRZ {
		t.Errorf("TDRZ: got %v, want %v", loaded.TDRZ, cfg.TDRZ)
	}
}
