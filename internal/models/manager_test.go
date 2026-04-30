package models

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCatalog_Unique(t *testing.T) {
	seen := map[string]bool{}
	for _, e := range Catalog() {
		if seen[e.ID] {
			t.Fatalf("duplicate id: %s", e.ID)
		}
		seen[e.ID] = true
		if e.URL == "" || e.RelPath == "" || e.Family == "" {
			t.Fatalf("entry %s missing required field", e.ID)
		}
	}
}

func TestCatalog_LLMDefaultActive(t *testing.T) {
	var actives int
	for _, e := range ByFamily(FamilyLLM) {
		if e.DefaultActive {
			actives++
		}
	}
	if actives != 1 {
		t.Fatalf("want exactly one default-active LLM, got %d", actives)
	}
}

func TestManager_GetAndStatus(t *testing.T) {
	body := strings.Repeat("x", 1024)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	tmp := t.TempDir()
	t.Setenv("WT_MODELS_DIR", tmp)
	t.Setenv("HOME", tmp)
	t.Setenv("APPDATA", tmp)
	t.Setenv("XDG_CONFIG_HOME", tmp)

	e := Entry{ID: "test-x", Family: FamilyLLM, URL: srv.URL, RelPath: "test-x.bin", SizeBytes: int64(len(body))}
	registerForTest(e)
	defer unregisterForTest(e.ID)

	mgr := NewManager()
	if got := mgr.Status(e.ID); got != StatusNotInstalled {
		t.Fatalf("status before download: got %s want %s", got, StatusNotInstalled)
	}

	var lastDone bool
	if err := mgr.Get(context.Background(), e.ID, func(p Progress) {
		if p.Done {
			lastDone = true
		}
	}); err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !lastDone {
		t.Fatalf("progress Done=true never seen")
	}
	if got := mgr.Status(e.ID); got != StatusInstalled {
		t.Fatalf("status after download: got %s want %s", got, StatusInstalled)
	}

	dst := PathFor(e)
	st, err := os.Stat(dst)
	if err != nil {
		t.Fatalf("stat downloaded: %v", err)
	}
	if st.Size() != int64(len(body)) {
		t.Fatalf("size: got %d want %d", st.Size(), len(body))
	}
}

func TestManager_SetActiveAndPersist(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("WT_MODELS_DIR", tmp)
	t.Setenv("HOME", tmp)
	t.Setenv("APPDATA", tmp)
	t.Setenv("XDG_CONFIG_HOME", tmp)

	e := Entry{ID: "test-active", Family: FamilyLLM, URL: "x", RelPath: "test-active.bin", SizeBytes: 1}
	registerForTest(e)
	defer unregisterForTest(e.ID)

	dst := PathFor(e)
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(dst, []byte("x"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	mgr := NewManager()
	if err := mgr.SetActive(e.ID); err != nil {
		t.Fatalf("SetActive: %v", err)
	}

	mgr2 := NewManager()
	if got := mgr2.Active(FamilyLLM); got != e.ID {
		t.Fatalf("active not persisted: got %q want %q", got, e.ID)
	}
}

func TestManager_DeleteClearsActive(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("WT_MODELS_DIR", tmp)
	t.Setenv("HOME", tmp)
	t.Setenv("APPDATA", tmp)
	t.Setenv("XDG_CONFIG_HOME", tmp)

	e := Entry{ID: "test-delete", Family: FamilyLLM, URL: "x", RelPath: "test-delete.bin", SizeBytes: 1}
	registerForTest(e)
	defer unregisterForTest(e.ID)

	dst := PathFor(e)
	_ = os.MkdirAll(filepath.Dir(dst), 0o755)
	_ = os.WriteFile(dst, []byte("x"), 0o644)

	mgr := NewManager()
	_ = mgr.SetActive(e.ID)
	if err := mgr.Delete(e.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if mgr.Active(FamilyLLM) == e.ID {
		t.Fatalf("active not cleared after delete")
	}
	if fileExists(dst) {
		t.Fatalf("file not removed")
	}
}
