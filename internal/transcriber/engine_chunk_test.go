package transcriber

import (
	"math"
	"strings"
	"testing"
)

func TestSplitChunks_BasicCoverage(t *testing.T) {
	dur := 90.0
	samples := make([]float32, int(dur*float64(WhisperSampleRate)))
	chunks := splitChunks(samples, 30.0)
	if len(chunks) < 3 {
		t.Fatalf("expected >=3 chunks for 90s @ 30s, got %d", len(chunks))
	}
	for i := range chunks {
		if i > 0 && chunks[i].StartSec < chunks[i-1].EndSec-0.001 {
			t.Fatalf("chunk %d overlaps previous: prev.end=%.3f cur.start=%.3f", i, chunks[i-1].EndSec, chunks[i].StartSec)
		}
		if chunks[i].EndSec <= chunks[i].StartSec {
			t.Fatalf("chunk %d non-positive duration", i)
		}
	}
	last := chunks[len(chunks)-1]
	if math.Abs(last.EndSec-dur) > 0.01 {
		t.Fatalf("final chunk should reach %g, got %g", dur, last.EndSec)
	}
}

func TestSnapBoundary_PrefersLowEnergyZone(t *testing.T) {
	t.Setenv("WT_CHUNK_NO_SNAP", "")
	totalSec := 60.0
	n := int(totalSec * float64(WhisperSampleRate))
	samples := make([]float32, n)
	for i := range samples {
		samples[i] = 0.5
	}
	zoneStart := samplesAt(28.5)
	zoneEnd := samplesAt(31.5)
	for i := zoneStart; i < zoneEnd && i < n; i++ {
		samples[i] = 0
	}
	target := samplesAt(30.0)
	got := snapBoundary(samples, target)
	if got < zoneStart || got > zoneEnd {
		t.Fatalf("snapBoundary should land in silent zone [%d,%d], got %d", zoneStart, zoneEnd, got)
	}
}

func TestSnapBoundary_DisabledByEnv(t *testing.T) {
	t.Setenv("WT_CHUNK_NO_SNAP", "1")
	samples := make([]float32, samplesAt(60.0))
	target := samplesAt(30.0)
	got := snapBoundary(samples, target)
	if got != target {
		t.Fatalf("env disable should be a no-op: got %d want %d", got, target)
	}
}

func TestLegacyResumeIndex_DerivesFromLastEndMs(t *testing.T) {
	chunks := []audioChunk{
		{StartSec: 0, EndSec: 30},
		{StartSec: 30, EndSec: 60},
		{StartSec: 60, EndSec: 90},
		{StartSec: 90, EndSec: 120},
	}
	cases := []struct {
		name    string
		lastMs  int64
		wantIdx int
	}{
		{"none", 0, 0},
		{"two-chunks-done", 60_000, 2},
		{"between-chunks-rerun-incomplete", 75_000, 2},
		{"all-done", 120_000, len(chunks)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := legacyResumeIndex(chunks, tc.lastMs); got != tc.wantIdx {
				t.Errorf("got %d want %d", got, tc.wantIdx)
			}
		})
	}
}

func TestChunkPlanID_StableIncludesSize(t *testing.T) {
	a := chunkPlanID(30.0, 4)
	b := chunkPlanID(30.0, 4)
	c := chunkPlanID(30.0, 5)
	d := chunkPlanID(20.0, 4)
	if a != b {
		t.Fatalf("plan id should be deterministic: %q != %q", a, b)
	}
	if a == c || a == d {
		t.Fatalf("plan id must differ on chunk count or size: %q vs %q vs %q", a, c, d)
	}
	if !strings.HasPrefix(a, "v2:") {
		t.Fatalf("plan id should be versioned, got %q", a)
	}
}
