//go:build integration

package diarizer

import (
	"context"
	"testing"
	"time"
)

func TestSherpaDiarize_TwoSpeakerSampleEn(t *testing.T) {
	backend, err := newSherpaDiarizer()
	if err != nil {
		t.Fatalf("sherpa-onnx unavailable: %v (run task build to stage binaries + EnsureSherpaModels)", err)
	}

	wav := getSherpaSample(t, "1-two-speakers-en.wav")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	segs, err := backend.Diarize(ctx, wav, 0, 0, nil)
	if err != nil {
		t.Fatalf("diarize: %v", err)
	}
	if len(segs) == 0 {
		t.Fatal("no segments returned")
	}

	speakers := map[int]bool{}
	for _, s := range segs {
		speakers[s.Speaker] = true
	}
	if len(speakers) != 2 {
		t.Errorf("expected 2 speakers, got %d (segments=%d)", len(speakers), len(segs))
	}
}

func TestSherpaDiarize_FourSpeakerSampleZh(t *testing.T) {
	backend, err := newSherpaDiarizer()
	if err != nil {
		t.Fatalf("sherpa-onnx unavailable: %v", err)
	}

	wav := getSherpaSample(t, "0-four-speakers-zh.wav")

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	segs, err := backend.Diarize(ctx, wav, 0, 0, nil)
	if err != nil {
		t.Fatalf("diarize: %v", err)
	}
	speakers := map[int]bool{}
	for _, s := range segs {
		speakers[s.Speaker] = true
	}
	if len(speakers) < 3 || len(speakers) > 5 {
		t.Errorf("expected ~4 speakers, got %d (segments=%d)", len(speakers), len(segs))
	}
}
