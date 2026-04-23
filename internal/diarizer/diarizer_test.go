package diarizer

import (
	"testing"
	"time"
)

func TestMapSegmentsToSpeakers_NoDiarization(t *testing.T) {
	segs := []TranscriptSegment{
		{Start: 0, End: 5 * time.Second, Text: "Hello"},
		{Start: 5 * time.Second, End: 10 * time.Second, Text: "World"},
	}

	result := MapSegmentsToSpeakers(segs, nil)
	if len(result) != 2 {
		t.Fatalf("expected 2 segments, got %d", len(result))
	}
	for _, r := range result {
		if r.Speaker != "SPEAKER_01" {
			t.Errorf("expected SPEAKER_01, got %s", r.Speaker)
		}
	}
}

func TestMapSegmentsToSpeakers_WithDiarization(t *testing.T) {
	segs := []TranscriptSegment{
		{Start: 0, End: 5 * time.Second, Text: "Hello"},
		{Start: 5 * time.Second, End: 10 * time.Second, Text: "World"},
		{Start: 10 * time.Second, End: 15 * time.Second, Text: "Again"},
	}

	diarSegs := []Segment{
		{Speaker: 0, StartSec: 0, EndSec: 5},
		{Speaker: 1, StartSec: 5, EndSec: 10},
		{Speaker: 0, StartSec: 10, EndSec: 15},
	}

	result := MapSegmentsToSpeakers(segs, diarSegs)
	if len(result) != 3 {
		t.Fatalf("expected 3 segments, got %d", len(result))
	}

	if result[0].Speaker != "SPEAKER_01" {
		t.Errorf("expected SPEAKER_01, got %s", result[0].Speaker)
	}
	if result[1].Speaker != "SPEAKER_02" {
		t.Errorf("expected SPEAKER_02, got %s", result[1].Speaker)
	}
	if result[2].Speaker != "SPEAKER_01" {
		t.Errorf("expected SPEAKER_01 (returning), got %s", result[2].Speaker)
	}
}

func TestMapSegmentsToSpeakers_GapFallsBackToNearest(t *testing.T) {
	segs := []TranscriptSegment{
		{Start: 20 * time.Second, End: 25 * time.Second, Text: "In the gap"},
	}

	diarSegs := []Segment{
		{Speaker: 0, StartSec: 0, EndSec: 10},
		{Speaker: 1, StartSec: 30, EndSec: 40},
	}

	result := MapSegmentsToSpeakers(segs, diarSegs)
	if len(result) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(result))
	}
	if result[0].Speaker != "SPEAKER_02" {
		t.Errorf("expected SPEAKER_02 (nearest by midpoint at 35s vs 5s), got %s", result[0].Speaker)
	}
}

func TestMapSegmentsToSpeakers_OverlappingDiarization(t *testing.T) {
	segs := []TranscriptSegment{
		{Start: 3 * time.Second, End: 8 * time.Second, Text: "Overlap"},
	}

	diarSegs := []Segment{
		{Speaker: 0, StartSec: 0, EndSec: 6},
		{Speaker: 1, StartSec: 4, EndSec: 10},
	}

	result := MapSegmentsToSpeakers(segs, diarSegs)
	if len(result) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(result))
	}
	if result[0].Speaker != "SPEAKER_02" {
		t.Errorf("expected SPEAKER_02 (more overlap 4s vs 3s), got %s", result[0].Speaker)
	}
}

func TestSpeakerForTime_TokenLevel(t *testing.T) {
	diarSegs := []Segment{
		{Speaker: 0, StartSec: 0, EndSec: 3},
		{Speaker: 1, StartSec: 3, EndSec: 6},
	}
	labels := SpeakerLabels(diarSegs)

	if got := SpeakerForTime(1.0, 2.0, diarSegs, labels); got != "SPEAKER_01" {
		t.Errorf("token in speaker 0 range: expected SPEAKER_01, got %s", got)
	}
	if got := SpeakerForTime(4.0, 5.0, diarSegs, labels); got != "SPEAKER_02" {
		t.Errorf("token in speaker 1 range: expected SPEAKER_02, got %s", got)
	}
}
