package transcriber

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	whisper "github.com/ggerganov/whisper.cpp/bindings/go/pkg/whisper"
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

func TestGroupWordsIntoUtterances_SmoothsSingleWordFlicker(t *testing.T) {

	words := []Word{
		{Text: "Now", Start: 0, End: 200, Speaker: "SPEAKER_01"},
		{Text: "what", Start: 200, End: 400, Speaker: "SPEAKER_01"},
		{Text: "do", Start: 400, End: 500, Speaker: "SPEAKER_02"},
		{Text: "you", Start: 500, End: 700, Speaker: "SPEAKER_01"},
		{Text: "think?", Start: 700, End: 1000, Speaker: "SPEAKER_01"},
	}
	utts := groupWordsIntoUtterances(words)
	if len(utts) != 1 {
		t.Fatalf("want 1 utterance after smoothing, got %d: %+v", len(utts), utts)
	}
	if utts[0].Speaker != "SPEAKER_01" || utts[0].Text != "Now what do you think?" {
		t.Errorf("utt0 = %+v", utts[0])
	}
}

func TestGroupWordsIntoUtterances_PreservesGenuineMultiWordTurn(t *testing.T) {

	words := []Word{
		{Text: "Tell", Start: 0, End: 200, Speaker: "SPEAKER_01"},
		{Text: "me", Start: 200, End: 400, Speaker: "SPEAKER_01"},
		{Text: "sports", Start: 500, End: 800, Speaker: "SPEAKER_02"},
		{Text: "football", Start: 800, End: 1100, Speaker: "SPEAKER_02"},
		{Text: "every", Start: 1100, End: 1400, Speaker: "SPEAKER_02"},
		{Text: "day", Start: 1400, End: 1700, Speaker: "SPEAKER_02"},
		{Text: "Okay", Start: 1900, End: 2200, Speaker: "SPEAKER_01"},
		{Text: "good.", Start: 2200, End: 2600, Speaker: "SPEAKER_01"},
	}
	utts := groupWordsIntoUtterances(words)
	if len(utts) != 3 {
		t.Fatalf("want 3 utterances, got %d: %+v", len(utts), utts)
	}
	if utts[0].Speaker != "SPEAKER_01" || utts[1].Speaker != "SPEAKER_02" || utts[2].Speaker != "SPEAKER_01" {
		t.Errorf("speakers = %s/%s/%s", utts[0].Speaker, utts[1].Speaker, utts[2].Speaker)
	}
	if !strings.Contains(utts[1].Text, "football") {
		t.Errorf("SPEAKER_02 turn lost football: %+v", utts[1])
	}
}

func TestGroupWordsIntoUtterances_SmoothsTwoWordFlicker(t *testing.T) {
	words := []Word{
		{Text: "hello", Start: 0, End: 200, Speaker: "SPEAKER_01"},
		{Text: "world", Start: 200, End: 400, Speaker: "SPEAKER_01"},
		{Text: "foo", Start: 400, End: 600, Speaker: "SPEAKER_02"},
		{Text: "bar", Start: 600, End: 800, Speaker: "SPEAKER_02"},
		{Text: "there", Start: 800, End: 1000, Speaker: "SPEAKER_01"},
		{Text: "end.", Start: 1000, End: 1200, Speaker: "SPEAKER_01"},
	}
	utts := groupWordsIntoUtterances(words)
	if len(utts) != 1 {
		t.Fatalf("want 1 utterance after 2-word flicker smoothing, got %d: %+v", len(utts), utts)
	}
	if utts[0].Speaker != "SPEAKER_01" {
		t.Errorf("utt0 speaker = %s, want SPEAKER_01", utts[0].Speaker)
	}
}

func TestGroupWordsIntoUtterances_SplitsOnSentenceEndSameSpeaker(t *testing.T) {

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

	words := []Word{
		{Text: "Hello world.", Start: 0, End: 1000, Speaker: "SPEAKER_01"},
		{Text: "How are you?", Start: 1000, End: 2500, Speaker: "SPEAKER_01"},
	}
	utts := groupWordsIntoUtterances(words)
	if len(utts) != 2 {
		t.Fatalf("want 2 utterances, got %d", len(utts))
	}
}

func TestCoalesceWhisperTokens_BPEContractionsAndSplits(t *testing.T) {

	ms := func(n int) time.Duration { return time.Duration(n) * time.Millisecond }
	tok := func(text string, start, end int) whisper.Token {
		return whisper.Token{Text: text, Start: ms(start), End: ms(end)}
	}
	in := []whisper.Token{
		tok(" Hello", 0, 400),
		tok(",", 400, 410),
		tok(" Mr", 500, 700),
		tok(" O", 700, 800),
		tok("'D", 800, 900),
		tok("on", 900, 1000),
		tok("nell", 1000, 1200),
		tok(".", 1200, 1210),
		tok(" I", 1300, 1400),
		tok("'m", 1400, 1500),
		tok(" Andrew", 1500, 1900),
		tok(".", 1900, 1910),
		tok(" F", 2000, 2100),
		tok("ul", 2100, 2200),
		tok("ham", 2200, 2400),
		tok(".", 2400, 2410),
	}
	got := coalesceWhisperTokens(in)

	wantTexts := []string{
		"Hello,", "Mr", "O'Donnell.", "I'm", "Andrew.", "Fulham.",
	}
	if len(got) != len(wantTexts) {
		t.Fatalf("got %d tokens, want %d: %+v", len(got), len(wantTexts), got)
	}
	for i, w := range wantTexts {
		if got[i].Text != w {
			t.Errorf("tok[%d].Text = %q, want %q", i, got[i].Text, w)
		}
	}

	endCheck := func(n int) time.Duration { return time.Duration(n) * time.Millisecond }
	if got[2].End != endCheck(1210) {
		t.Errorf("O'Donnell. end = %v, want 1.21s", got[2].End)
	}
	if got[2].Start != endCheck(700) {
		t.Errorf("O'Donnell. start = %v, want 0.7s", got[2].Start)
	}

	joined := joinWords(func() []string {
		out := make([]string, len(got))
		for i, t := range got {
			out[i] = t.Text
		}
		return out
	}())
	const want = "Hello, Mr O'Donnell. I'm Andrew. Fulham."
	if joined != want {
		t.Errorf("joined = %q, want %q", joined, want)
	}
}

func TestCoalesceWhisperTokens_EmptyAndAllContinuation(t *testing.T) {
	if got := coalesceWhisperTokens(nil); got != nil {
		t.Errorf("nil input: got %+v, want nil", got)
	}

	in := []whisper.Token{{Text: "Hello", End: 500 * time.Millisecond}}
	got := coalesceWhisperTokens(in)
	if len(got) != 1 || got[0].Text != "Hello" {
		t.Fatalf("got %+v, want one word \"Hello\"", got)
	}
}
