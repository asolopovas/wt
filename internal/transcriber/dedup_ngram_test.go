package transcriber

import (
	"strings"
	"testing"
	"time"

	"github.com/asolopovas/wt/internal/diarizer"
)

func mkTokens(words ...string) []diarizer.TokenData {
	out := make([]diarizer.TokenData, len(words))
	for i, w := range words {
		out[i] = diarizer.TokenData{
			Text:  w,
			Start: time.Duration(i) * 100 * time.Millisecond,
			End:   time.Duration(i+1) * 100 * time.Millisecond,
		}
	}
	return out
}

func tokenWords(toks []diarizer.TokenData) []string {
	out := make([]string, len(toks))
	for i, t := range toks {
		out[i] = t.Text
	}
	return out
}

func TestCollapseRepeats_DropsRepeatedBigram(t *testing.T) {
	in := mkTokens("I", "think", "I", "think", "I", "think", "this", "is", "hard")
	got := collapseRepeats(in)
	want := []string{"I", "think", "this", "is", "hard"}
	if got, want := strings.Join(tokenWords(got), " "), strings.Join(want, " "); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestCollapseRepeats_DropsRepeatedTrigram(t *testing.T) {
	in := mkTokens("thank", "you", "very", "thank", "you", "very", "much")
	got := collapseRepeats(in)
	if w := strings.Join(tokenWords(got), " "); w != "thank you very much" {
		t.Errorf("got %q, want %q", w, "thank you very much")
	}
}

func TestCollapseRepeats_PreservesGenuineRepeats(t *testing.T) {
	in := mkTokens("very", "very", "good")
	got := collapseRepeats(in)
	if w := strings.Join(tokenWords(got), " "); w != "very very good" {
		t.Errorf("genuine 2x repeat should survive: got %q", w)
	}
}

func TestCollapseRepeats_DropsLongUnigramRun(t *testing.T) {
	in := mkTokens("yes", "yes", "yes", "yes", "yes", "yes", "okay")
	got := collapseRepeats(in)
	if w := strings.Join(tokenWords(got), " "); w != "yes okay" {
		t.Errorf("got %q, want %q", w, "yes okay")
	}
}

func TestCollapseRepeats_HonorsCanonicalWordEquality(t *testing.T) {
	in := mkTokens("Thank", "you,", "thank", "you,", "thank", "you.")
	got := collapseRepeats(in)
	if len(got) != 2 {
		t.Errorf("case+punct-folded match should collapse 3 copies to 1: got %v", tokenWords(got))
	}
}

func TestDedupRepeatedNgrams_AcrossSegments(t *testing.T) {
	segs := []diarizer.TranscriptSegment{
		{Tokens: mkTokens("hello", "world", "thank", "you")},
		{Tokens: mkTokens("thank", "you", "next", "topic")},
	}
	for i := range segs {
		segs[i] = rebuildSegmentFromTokens(segs[i])
	}
	got := DedupRepeatedNgrams(segs)
	if len(got) != 2 {
		t.Fatalf("want 2 segs, got %d", len(got))
	}
	if w := tokenWords(got[1].Tokens); strings.Join(w, " ") != "next topic" {
		t.Errorf("seam dedup failed: got %v", w)
	}
}

func TestDedupRepeatedNgrams_RespectsEnvDisable(t *testing.T) {
	t.Setenv("WT_NO_REPEAT_DEDUP", "1")
	segs := []diarizer.TranscriptSegment{
		{Tokens: mkTokens("yes", "yes", "yes", "yes", "yes", "yes")},
	}
	got := DedupRepeatedNgrams(segs)
	if len(got[0].Tokens) != 6 {
		t.Errorf("env disable should be a no-op: got %d tokens", len(got[0].Tokens))
	}
}

func TestDedupRepeatedNgrams_TextOnlyPathCollapsesWhisperHallucination(t *testing.T) {
	noString := strings.TrimSpace(strings.Repeat("No, ", 38))
	segs := []diarizer.TranscriptSegment{
		{Text: noString + " hello there"},
	}
	got := DedupRepeatedNgrams(segs)
	words := strings.Fields(got[0].Text)
	if len(words) > 5 {
		t.Errorf("expected ~3-4 words after collapse, got %d: %q", len(words), got[0].Text)
	}
}

func TestRebuildSegmentFromTokens_RecomputesTextAndBounds(t *testing.T) {
	seg := diarizer.TranscriptSegment{Tokens: mkTokens("a", "b", "c"), Text: "stale"}
	got := rebuildSegmentFromTokens(seg)
	if got.Text != "a b c" {
		t.Errorf("text: got %q want %q", got.Text, "a b c")
	}
	if got.Start != 0 {
		t.Errorf("start: got %v want 0", got.Start)
	}
	if got.End != 300*time.Millisecond {
		t.Errorf("end: got %v want 300ms", got.End)
	}
}
