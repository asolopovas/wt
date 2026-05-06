package cache

import (
	"testing"
	"time"

	"github.com/asolopovas/wt/internal/diarizer"
)

func TestPartial_V2RoundTripIncludesChunkPlan(t *testing.T) {
	_ = redirectAppDir(t)

	key := "abc123"
	in := Partial{
		Segments: []diarizer.TranscriptSegment{
			{Start: 0, End: time.Second, Text: "hi"},
		},
		LastEndMs:       1000,
		AudioDurMs:      60_000,
		CompletedChunks: 7,
		ChunkPlan:       "v2:sec=30.00:n=74",
	}
	if err := SavePartial(key, in); err != nil {
		t.Fatalf("SavePartial: %v", err)
	}
	out, ok := LoadPartial(key)
	if !ok {
		t.Fatal("LoadPartial returned !ok")
	}
	if out.CompletedChunks != in.CompletedChunks {
		t.Errorf("CompletedChunks: got %d want %d", out.CompletedChunks, in.CompletedChunks)
	}
	if out.ChunkPlan != in.ChunkPlan {
		t.Errorf("ChunkPlan: got %q want %q", out.ChunkPlan, in.ChunkPlan)
	}
	if out.LastEndMs != in.LastEndMs {
		t.Errorf("LastEndMs: got %d want %d", out.LastEndMs, in.LastEndMs)
	}
}

func TestPartial_LoadsLegacyWithoutChunkPlan(t *testing.T) {
	_ = redirectAppDir(t)

	key := "legacy"
	in := Partial{
		Segments:   []diarizer.TranscriptSegment{{Start: 0, End: time.Second, Text: "x"}},
		LastEndMs:  500,
		AudioDurMs: 30_000,
	}
	if err := SavePartial(key, in); err != nil {
		t.Fatalf("SavePartial: %v", err)
	}
	out, ok := LoadPartial(key)
	if !ok {
		t.Fatal("legacy partial should still load")
	}
	if out.CompletedChunks != 0 {
		t.Errorf("CompletedChunks: got %d want 0 for legacy", out.CompletedChunks)
	}
	if out.ChunkPlan != "" {
		t.Errorf("ChunkPlan: got %q want empty for legacy", out.ChunkPlan)
	}
	if out.LastEndMs != in.LastEndMs {
		t.Errorf("LastEndMs: got %d want %d", out.LastEndMs, in.LastEndMs)
	}
}

func TestPartial_RejectsEmpty(t *testing.T) {
	_ = redirectAppDir(t)
	key := "empty"
	if err := SavePartial(key, Partial{}); err != nil {
		t.Fatalf("SavePartial empty: %v", err)
	}
	if _, ok := LoadPartial(key); ok {
		t.Fatal("empty partial should not load")
	}
}
