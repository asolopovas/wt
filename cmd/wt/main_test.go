package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pterm/pterm"
)

func TestExpandFiles_ValidGlob(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"a.wav", "b.wav", "c.txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("test"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	pattern := filepath.Join(dir, "*.wav")
	paths, err := expandFiles([]string{pattern})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(paths) != 2 {
		t.Errorf("expected 2 matches, got %d", len(paths))
	}
	for _, p := range paths {
		if !filepath.IsAbs(p) {
			t.Errorf("expected absolute path, got %q", p)
		}
	}
}

func TestExpandFiles_InvalidGlob(t *testing.T) {
	_, err := expandFiles([]string{"[invalid"})
	if err == nil {
		t.Fatal("expected error for invalid glob pattern")
	}
}

func TestExpandFiles_DirectFile(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "test.ogg")
	if err := os.WriteFile(tmp, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}

	paths, err := expandFiles([]string{tmp})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(paths) != 1 {
		t.Fatalf("expected 1 path, got %d", len(paths))
	}
}

func TestExpandFiles_NonexistentSkipped(t *testing.T) {
	pterm.DisableOutput()
	defer pterm.EnableOutput()
	paths, err := expandFiles([]string{"/nonexistent/file.wav"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(paths) != 0 {
		t.Errorf("expected 0 paths for nonexistent file, got %d", len(paths))
	}
}

func TestRun_NoArgs(t *testing.T) {
	err := run("", "base", "", 1, 0, false, false, false, nil)
	if err != nil {
		t.Errorf("expected nil error for no-args usage, got: %v", err)
	}
}

func TestRun_NoMatchingFiles(t *testing.T) {
	pterm.DisableOutput()
	defer pterm.EnableOutput()
	err := run("", "base", "", 1, 0, false, false, false, []string{"/nonexistent/*.wav"})
	if err == nil {
		t.Fatal("expected error when no files match")
	}
}
