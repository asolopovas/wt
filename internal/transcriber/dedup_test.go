package transcriber

import (
	"testing"
	"time"

	"github.com/asolopovas/wt/internal/diarizer"
)

func TestDeduplicateSegments_MergesAdjacentDuplicates(t *testing.T) {
	segs := []diarizer.TranscriptSegment{
		{Start: 0, End: time.Second, Text: "hello"},
		{Start: time.Second, End: 2 * time.Second, Text: " hello "},
		{Start: 2 * time.Second, End: 3 * time.Second, Text: "world"},
	}
	got := DeduplicateSegments(segs)
	if len(got) != 2 {
		t.Fatalf("len=%d want 2 (first hello extended, then world)", len(got))
	}
	if got[0].End != 2*time.Second {
		t.Errorf("first.End=%v want 2s (merged)", got[0].End)
	}
	if got[1].Text != "world" {
		t.Errorf("second.Text=%q want world", got[1].Text)
	}
}

func TestDeduplicateSegments_PreservesNonDuplicates(t *testing.T) {
	segs := []diarizer.TranscriptSegment{
		{Start: 0, End: time.Second, Text: "a"},
		{Start: time.Second, End: 2 * time.Second, Text: "b"},
		{Start: 2 * time.Second, End: 3 * time.Second, Text: "c"},
	}
	got := DeduplicateSegments(segs)
	if len(got) != 3 {
		t.Fatalf("len=%d want 3", len(got))
	}
}

func TestDeduplicateSegments_ShortInput(t *testing.T) {
	if got := DeduplicateSegments(nil); got != nil {
		t.Error("nil input should yield nil")
	}
	one := []diarizer.TranscriptSegment{{Text: "x"}}
	if got := DeduplicateSegments(one); len(got) != 1 {
		t.Errorf("len=%d want 1", len(got))
	}
}
