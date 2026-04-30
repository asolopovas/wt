//go:build !android

package transcriber

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseFFmpegDuration(t *testing.T) {
	tests := []struct {
		name   string
		stderr string
		want   int64
	}{
		{
			"typical ffmpeg -i output",
			"Input #0, mov,mp4,m4a, ...\n  Duration: 00:01:23.45, start: 0.000000, bitrate: 128 kb/s\n",
			83450,
		},
		{
			"hours scale",
			"Duration: 02:30:00.00, start: ...",
			(2*3600 + 30*60) * 1000,
		},
		{
			"missing duration line",
			"Input #0, mov\n  bitrate: 128 kb/s",
			0,
		},
		{
			"malformed hms",
			"Duration: 1:2, ...",
			0,
		},
		{
			"non-numeric components",
			"Duration: aa:bb:cc.dd, ...",
			0,
		},
		{
			"zero duration",
			"Duration: 00:00:00.00, ...",
			0,
		},
		{
			"no comma terminator",
			"Duration: 00:01:23.45 start: 0",
			0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseFFmpegDuration(tt.stderr)
			if got != tt.want {
				t.Errorf("got %d, want %d", got, tt.want)
			}
		})
	}
}

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
