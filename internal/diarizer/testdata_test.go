//go:build integration

package diarizer

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

const sherpaSampleBase = "https://github.com/k2-fsa/sherpa-onnx/releases/download/speaker-segmentation-models/"

func sampleCacheDir(t testing.TB) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(thisFile), "..", "..", "samples", "diarization", "sherpa")
}

func getSherpaSample(t testing.TB, name string) string {
	t.Helper()
	dir := sampleCacheDir(t)
	path := filepath.Join(dir, name)
	if st, err := os.Stat(path); err == nil && st.Size() > 0 {
		return path
	}
	if testing.Short() {
		t.Skipf("sample %s not cached (re-run without -short to download)", name)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Get(sherpaSampleBase + name)
	if err != nil {
		t.Fatalf("downloading %s: %v", name, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("downloading %s: HTTP %d", name, resp.StatusCode)
	}

	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		t.Fatalf("writing %s: %v", name, err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		t.Fatal(err)
	}
	if err := os.Rename(tmp, path); err != nil {
		t.Fatal(err)
	}
	return path
}
