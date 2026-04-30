package shared

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
)

func TestDownloadFile_Success(t *testing.T) {
	body := bytes.Repeat([]byte("abcdefgh"), 4096)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", strconv.Itoa(len(body)))
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	dst := filepath.Join(t.TempDir(), "out.bin")
	var lastDownloaded, lastTotal int64
	prog := func(d, total int64) {
		lastDownloaded = d
		lastTotal = total
	}

	if err := DownloadFile(dst, srv.URL, prog); err != nil {
		t.Fatalf("DownloadFile: %v", err)
	}
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, body) {
		t.Errorf("body mismatch: got %d bytes want %d", len(got), len(body))
	}
	if lastDownloaded != lastTotal || lastTotal != int64(len(body)) {
		t.Errorf("final progress: downloaded=%d total=%d want %d/%d", lastDownloaded, lastTotal, len(body), len(body))
	}
	if _, err := os.Stat(dst + ".part"); !os.IsNotExist(err) {
		t.Errorf(".part file should be gone, stat err=%v", err)
	}
}

func TestDownloadFile_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "nope", http.StatusNotFound)
	}))
	defer srv.Close()

	dst := filepath.Join(t.TempDir(), "out.bin")
	err := DownloadFile(dst, srv.URL, nil)
	if err == nil {
		t.Fatal("expected error for 404")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("error should mention 404, got %v", err)
	}
}

func TestDownloadFile_ResumesPartial(t *testing.T) {
	body := make([]byte, 4096)
	if _, err := rand.Read(body); err != nil {
		t.Fatal(err)
	}

	var rangeRequests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if rng := r.Header.Get("Range"); rng != "" {
			rangeRequests.Add(1)
			var start int64
			if _, err := fmt.Sscanf(rng, "bytes=%d-", &start); err != nil {
				http.Error(w, "bad range", http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Length", strconv.Itoa(len(body)-int(start)))
			w.WriteHeader(http.StatusPartialContent)
			_, _ = w.Write(body[start:])
			return
		}
		w.Header().Set("Content-Length", strconv.Itoa(len(body)))
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	dst := filepath.Join(t.TempDir(), "out.bin")
	tmp := dst + ".part"
	if err := os.WriteFile(tmp, body[:1024], 0o644); err != nil {
		t.Fatal(err)
	}

	if err := DownloadFile(dst, srv.URL, nil); err != nil {
		t.Fatalf("DownloadFile: %v", err)
	}
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, body) {
		t.Errorf("body mismatch: got %d bytes want %d", len(got), len(body))
	}
	if rangeRequests.Load() != 1 {
		t.Errorf("expected exactly 1 range request, got %d", rangeRequests.Load())
	}
}
