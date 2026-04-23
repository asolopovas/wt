package transcriber

import (
	"os"
	"path/filepath"
	"testing"

	shared "github.com/asolopovas/wt/internal"
)

func TestModelsDir(t *testing.T) {
	dir := shared.ModelsDir()
	if dir == "" {
		t.Fatal("ModelsDir returned empty string")
	}
	if !filepath.IsAbs(dir) {
		t.Logf("ModelsDir is relative (home dir lookup failed): %s", dir)
	}
	if filepath.Base(dir) != "models" {
		t.Errorf("expected models dir to end with 'models', got %q", dir)
	}
}

func TestResolveModelPath_ExplicitPath(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "test-model.bin")
	if err := os.WriteFile(tmp, []byte("fake model"), 0o644); err != nil {
		t.Fatal(err)
	}

	path, err := ResolveModelPath("base", tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if path != tmp {
		t.Errorf("expected %q, got %q", tmp, path)
	}
}

func TestResolveModelPath_ExplicitPathNotFound(t *testing.T) {
	_, err := ResolveModelPath("base", "/nonexistent/model.bin")
	if err == nil {
		t.Fatal("expected error for nonexistent explicit model path")
	}
}

func TestResolveModelPath_UnknownModelSize(t *testing.T) {
	_, err := ResolveModelPath("nonexistent-model-size", "")
	if err == nil {
		t.Fatal("expected error for unknown model size")
	}
}

func TestModelFiles_AllEntriesNonEmpty(t *testing.T) {
	for size, filename := range ModelFiles {
		if size == "" {
			t.Error("empty model size key")
		}
		if filename == "" {
			t.Errorf("empty filename for model size %q", size)
		}
		if filepath.Ext(filename) != ".bin" {
			t.Errorf("expected .bin extension for %q, got %q", size, filename)
		}
	}
}
