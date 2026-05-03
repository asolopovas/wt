package transcriber

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var models = []string{"tiny", "base", "small", "medium", "turbo"}

func TestOutputFilename(t *testing.T) {
	name := OutputFilename("test.opus", "turbo")
	if !strings.HasPrefix(name, "test.opus_turbo_") {
		t.Errorf("expected prefix test.opus_turbo_, got %q", name)
	}
	if !strings.HasSuffix(name, ".json") {
		t.Errorf("expected .json suffix, got %q", name)
	}
	stamp := strings.TrimPrefix(name, "test.opus_turbo_")
	stamp = strings.TrimSuffix(stamp, ".json")
	if _, err := time.Parse("2006-01-02_150405", stamp); err != nil {
		t.Errorf("invalid timestamp in filename %q: %v", name, err)
	}
}

func TestOutputFilename_DifferentModels(t *testing.T) {
	for _, model := range models {
		name := OutputFilename("audio.wav", model)
		if !strings.Contains(name, "_"+model+"_") {
			t.Errorf("filename %q does not contain model %q", name, model)
		}
	}
}

func TestWriteAndReadJSON(t *testing.T) {
	tr := &Transcript{
		Language:   "en",
		DurationMs: 5000,
		Utterances: []Utterance{
			{Start: 0, End: 2500, Speaker: "A", Text: "Hello world."},
			{Start: 2500, End: 5000, Speaker: "B", Text: "Hi there."},
		},
		Words: []Word{
			{Text: "Hello", Start: 0, End: 1000, Speaker: "A", Confidence: 0.99},
			{Text: "world.", Start: 1000, End: 2500, Speaker: "A", Confidence: 0.95},
			{Text: "Hi", Start: 2500, End: 3500, Speaker: "B", Confidence: 0.98},
			{Text: "there.", Start: 3500, End: 5000, Speaker: "B", Confidence: 0.97},
		},
	}

	outPath := filepath.Join(t.TempDir(), "test_output.json")
	actual, err := WriteJSON(outPath, tr)
	if err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}
	if actual != outPath {
		t.Errorf("WriteJSON returned %q, want %q", actual, outPath)
	}

	data, err := os.ReadFile(actual)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	var loaded Transcript
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if loaded.Language != tr.Language {
		t.Errorf("language = %q, want %q", loaded.Language, tr.Language)
	}
	if loaded.DurationMs != tr.DurationMs {
		t.Errorf("duration_ms = %d, want %d", loaded.DurationMs, tr.DurationMs)
	}
	if len(loaded.Utterances) != len(tr.Utterances) {
		t.Fatalf("utterance count = %d, want %d", len(loaded.Utterances), len(tr.Utterances))
	}
	for i, u := range loaded.Utterances {
		if u != tr.Utterances[i] {
			t.Errorf("utterance[%d] = %+v, want %+v", i, u, tr.Utterances[i])
		}
	}
	if len(loaded.Words) != len(tr.Words) {
		t.Fatalf("word count = %d, want %d", len(loaded.Words), len(tr.Words))
	}
}

func TestGroupWordsIntoUtterances_BasicSpeakerChange(t *testing.T) {
	words := []Word{
		{Text: "Hello", Start: 0, End: 500, Speaker: "SPEAKER_01"},
		{Text: "there.", Start: 500, End: 1000, Speaker: "SPEAKER_01"},
		{Text: "Hi", Start: 1100, End: 1400, Speaker: "SPEAKER_02"},
		{Text: "back.", Start: 1400, End: 1800, Speaker: "SPEAKER_02"},
	}
	utts := groupWordsIntoUtterances(words)
	if len(utts) != 2 {
		t.Fatalf("want 2 utterances, got %d: %+v", len(utts), utts)
	}
	if utts[0].Speaker != "SPEAKER_01" || utts[0].Text != "Hello there." {
		t.Errorf("utt0 = %+v", utts[0])
	}
	if utts[1].Speaker != "SPEAKER_02" || utts[1].Text != "Hi back." {
		t.Errorf("utt1 = %+v", utts[1])
	}
}

func TestGroupWordsIntoUtterances_RealignsMidSentenceFlicker(t *testing.T) {
	// Mid-sentence speaker flicker: a single word in the middle of a
	// SPEAKER_01 sentence is mislabelled SPEAKER_02. Punctuation-based
	// realignment should majority-vote it back to SPEAKER_01.
	words := []Word{
		{Text: "Now", Start: 0, End: 200, Speaker: "SPEAKER_01"},
		{Text: "what", Start: 200, End: 400, Speaker: "SPEAKER_01"},
		{Text: "do", Start: 400, End: 500, Speaker: "SPEAKER_02"}, // flicker
		{Text: "you", Start: 500, End: 700, Speaker: "SPEAKER_01"},
		{Text: "think?", Start: 700, End: 1000, Speaker: "SPEAKER_01"},
		{Text: "Well,", Start: 1100, End: 1400, Speaker: "SPEAKER_02"},
		{Text: "interesting.", Start: 1400, End: 2000, Speaker: "SPEAKER_02"},
	}
	utts := groupWordsIntoUtterances(words)
	if len(utts) != 2 {
		t.Fatalf("want 2 utterances after realignment, got %d: %+v", len(utts), utts)
	}
	if utts[0].Speaker != "SPEAKER_01" || !strings.Contains(utts[0].Text, "do") {
		t.Errorf("utt0 = %+v (expected the flicker absorbed into SPEAKER_01)", utts[0])
	}
	if utts[1].Speaker != "SPEAKER_02" {
		t.Errorf("utt1 = %+v", utts[1])
	}
}

func TestGroupWordsIntoUtterances_SplitsOnSentenceEndSameSpeaker(t *testing.T) {
	// One speaker, two consecutive sentences should produce two
	// utterances split on the period.
	words := []Word{
		{Text: "First", Start: 0, End: 200, Speaker: "SPEAKER_01"},
		{Text: "sentence.", Start: 200, End: 600, Speaker: "SPEAKER_01"},
		{Text: "Second", Start: 700, End: 900, Speaker: "SPEAKER_01"},
		{Text: "one.", Start: 900, End: 1200, Speaker: "SPEAKER_01"},
	}
	utts := groupWordsIntoUtterances(words)
	if len(utts) != 2 {
		t.Fatalf("want 2 utterances, got %d: %+v", len(utts), utts)
	}
	if utts[0].Text != "First sentence." || utts[1].Text != "Second one." {
		t.Errorf("got: %+v / %+v", utts[0], utts[1])
	}
}

func TestGroupWordsIntoUtterances_NoSpeakerLabels(t *testing.T) {
	// Whisper-style: sentence-level segments collapsed into "words" with
	// no diarization. Should produce one utterance per sentence.
	words := []Word{
		{Text: "Hello world.", Start: 0, End: 1000, Speaker: "SPEAKER_01"},
		{Text: "How are you?", Start: 1000, End: 2500, Speaker: "SPEAKER_01"},
	}
	utts := groupWordsIntoUtterances(words)
	if len(utts) != 2 {
		t.Fatalf("want 2 utterances, got %d", len(utts))
	}
}
