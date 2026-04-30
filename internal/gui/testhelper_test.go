package gui

import (
	"os"
	"path/filepath"
	"testing"
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
