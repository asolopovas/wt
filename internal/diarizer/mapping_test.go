package diarizer

import (
	"testing"
	"time"
)

func TestSpeakerTurnSegments_AlternatesAfterTurn(t *testing.T) {
	segs := []TranscriptSegment{
		{Start: 0, End: 1 * time.Second, SpeakerTurnNext: false},
		{Start: 1 * time.Second, End: 2 * time.Second, SpeakerTurnNext: true},
		{Start: 2 * time.Second, End: 3 * time.Second, SpeakerTurnNext: false},
	}

	got := SpeakerTurnSegments(segs)
	if len(got) != 3 {
		t.Fatalf("len=%d want 3", len(got))
	}

	if got[0].Speaker != 0 || got[1].Speaker != 0 || got[2].Speaker != 1 {
		t.Fatalf("speakers=%v want [0 0 1]", []int{got[0].Speaker, got[1].Speaker, got[2].Speaker})
	}
}

func TestSpeakerTurnSegments_Empty(t *testing.T) {
	if got := SpeakerTurnSegments(nil); got != nil {
		t.Fatalf("got %v want nil", got)
	}
}
