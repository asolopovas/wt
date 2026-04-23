package transcriber

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadAudioSamples_FileNotFound(t *testing.T) {
	_, err := LoadAudioSamples("/nonexistent/path/audio.wav")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestLoadAudioSamples_InvalidWAVFallback(t *testing.T) {
	tmp, err := os.CreateTemp(t.TempDir(), "bad-*.wav")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tmp.Write([]byte("not a real wav file")); err != nil {
		t.Fatal(err)
	}
	_ = tmp.Close()

	_, err = LoadAudioSamples(tmp.Name())
	if err == nil {
		t.Fatal("expected error for invalid WAV without ffmpeg")
	}
}

func TestReadPCM16WAV_InvalidFile(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "empty.wav")
	if err := os.WriteFile(tmp, []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := readPCM16WAV(tmp)
	if err == nil {
		t.Fatal("expected error from empty WAV file")
	}
}

func TestReadPCM16WAV_NotRIFF(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "bad.wav")
	if err := os.WriteFile(tmp, []byte("not a RIFF file at all!"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := readPCM16WAV(tmp)
	if err == nil {
		t.Fatal("expected error from non-RIFF file")
	}
}

func TestAudioCacheKey_Deterministic(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "test.m4a")
	if err := os.WriteFile(tmp, []byte("fake audio data"), 0o644); err != nil {
		t.Fatal(err)
	}

	key1, err := AudioCacheKey(tmp)
	if err != nil {
		t.Fatalf("first call failed: %v", err)
	}
	key2, err := AudioCacheKey(tmp)
	if err != nil {
		t.Fatalf("second call failed: %v", err)
	}

	if key1 != key2 {
		t.Errorf("cache key not deterministic: %q != %q", key1, key2)
	}
	if filepath.Ext(key1) != ".wav" {
		t.Errorf("expected .wav extension, got %q", key1)
	}
}

func TestAudioCacheKey_DifferentFiles(t *testing.T) {
	dir := t.TempDir()
	f1 := filepath.Join(dir, "a.m4a")
	f2 := filepath.Join(dir, "b.m4a")
	if err := os.WriteFile(f1, []byte("data1"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f2, []byte("data2different"), 0o644); err != nil {
		t.Fatal(err)
	}

	k1, _ := AudioCacheKey(f1)
	k2, _ := AudioCacheKey(f2)
	if k1 == k2 {
		t.Error("different files should produce different cache keys")
	}
}

func TestAudioCacheKey_NonexistentFile(t *testing.T) {
	_, err := AudioCacheKey("/nonexistent/file.m4a")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}
