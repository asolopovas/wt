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
