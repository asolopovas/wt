package transcriber

import (
	"math"
	"path/filepath"
	"testing"
)

func TestWriteAndReadPCM16WAV_RoundTrip(t *testing.T) {
	in := []float32{0, 0.25, -0.25, 0.5, -0.5, 1.0, -1.0}
	path := filepath.Join(t.TempDir(), "round.wav")

	if err := WritePCM16WAV(path, in, WhisperSampleRate); err != nil {
		t.Fatalf("WritePCM16WAV: %v", err)
	}

	out, err := readPCM16WAV(path)
	if err != nil {
		t.Fatalf("readPCM16WAV: %v", err)
	}
	if len(out) != len(in) {
		t.Fatalf("len=%d want %d", len(out), len(in))
	}
	const tol = 1.0 / 32767.0 * 2
	for i, v := range in {
		if math.Abs(float64(out[i]-v)) > tol {
			t.Errorf("sample[%d]=%v want %v (tol %v)", i, out[i], v, tol)
		}
	}
}

func TestReadPCM16WAV_RejectsWrongSampleRate(t *testing.T) {
	in := []float32{0, 0.1, -0.1}
	path := filepath.Join(t.TempDir(), "wrongrate.wav")

	if err := WritePCM16WAV(path, in, 8000); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := readPCM16WAV(path); err == nil {
		t.Error("expected error on non-16k WAV")
	}
}
