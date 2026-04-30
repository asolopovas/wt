package transcribe

import (
	"bytes"
	"encoding/csv"
	"strings"
	"testing"
	"time"

	"github.com/asolopovas/wt/internal/transcriber"
)

func TestExportBaseName(t *testing.T) {
	tests := []struct {
		name, model, want string
	}{
		{"audio.m4a", "tiny", "audio-tiny"},
		{"path/to/clip.opus", "turbo", "clip-turbo"},
		{"audio.wav", "", "audio"},
		{"audio.wav", "  ", "audio"},
	}
	for _, tt := range tests {
		got := exportBaseName(tt.name, tt.model)
		if got != tt.want {
			t.Errorf("exportBaseName(%q,%q)=%q want %q", tt.name, tt.model, got, tt.want)
		}
	}
}

func TestFormatAbsoluteTimestamp(t *testing.T) {
	start := time.Date(2026, 1, 2, 9, 0, 0, 0, time.UTC)
	got := formatAbsoluteTimestamp(int64(time.Hour/time.Millisecond), start)
	want := start.Add(time.Hour).Format(startTimeLayout)
	if got != want {
		t.Errorf("got=%q want=%q", got, want)
	}
}

func TestWriteCSV(t *testing.T) {
	tr := &transcriber.Transcript{
		Utterances: []transcriber.Utterance{
			{Start: 0, End: 1000, Speaker: "A", Text: "Hello, world"},
			{Start: 1000, End: 2000, Speaker: "B", Text: "  Hi  "},
		},
	}
	var buf bytes.Buffer
	if err := writeCSV(&buf, tr, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("writeCSV: %v", err)
	}
	r := csv.NewReader(&buf)
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("rows=%d want 3", len(rows))
	}
	if rows[0][0] != "start" {
		t.Errorf("header[0]=%q", rows[0][0])
	}
	if rows[1][3] != "Hello, world" {
		t.Errorf("comma-text not preserved: %q", rows[1][3])
	}
	if rows[2][3] != "Hi" {
		t.Errorf("expected trimmed text, got %q", rows[2][3])
	}
}

func TestWriteText(t *testing.T) {
	tr := &transcriber.Transcript{
		Utterances: []transcriber.Utterance{
			{Start: 0, End: 1000, Speaker: "A", Text: " hello "},
		},
	}
	var buf bytes.Buffer
	if err := writeText(&buf, tr, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "A: hello") {
		t.Errorf("unexpected output: %q", out)
	}
	if !strings.HasSuffix(out, "\n") {
		t.Errorf("missing trailing newline")
	}
}

func TestWriteExport_UnknownFormat(t *testing.T) {
	tr := &transcriber.Transcript{}
	err := writeExport(&bytes.Buffer{}, tr, exportFormat{ext: "xml"}, time.Now())
	if err == nil {
		t.Fatal("expected error for unknown format")
	}
}

func TestWriteExport_JSONPath(t *testing.T) {
	tr := &transcriber.Transcript{
		Model:    "tiny",
		Language: "en",
		Utterances: []transcriber.Utterance{
			{Start: 0, End: 1000, Speaker: "A", Text: "hi"},
		},
	}
	var buf bytes.Buffer
	if err := writeExport(&buf, tr, exportFormat{ext: "json"}, time.Now()); err != nil {
		t.Fatalf("writeExport: %v", err)
	}
	if !strings.Contains(buf.String(), `"model": "tiny"`) {
		t.Errorf("output missing model field: %q", buf.String())
	}
	if !strings.Contains(buf.String(), `"language": "en"`) {
		t.Errorf("output missing language field")
	}
}

func TestPanelRenamedTranscript_NoOp(t *testing.T) {
	p := &Panel{}
	tr := &transcriber.Transcript{
		Utterances: []transcriber.Utterance{{Speaker: "SPEAKER_01", Text: "x"}},
	}
	got := p.renamedTranscript(tr)
	if got != tr {
		t.Error("no renames: should return same pointer (no copy)")
	}
}

func TestPanelRenamedTranscript_Renames(t *testing.T) {
	p := &Panel{speakerRenames: map[string]string{"SPEAKER_01": "Alice"}}
	tr := &transcriber.Transcript{
		Utterances: []transcriber.Utterance{
			{Speaker: "SPEAKER_01", Text: "hi"},
			{Speaker: "SPEAKER_02", Text: "yo"},
		},
		Words: []transcriber.Word{
			{Speaker: "SPEAKER_01", Text: "hi"},
		},
	}
	got := p.renamedTranscript(tr)
	if got == tr {
		t.Error("expected new transcript pointer, got same")
	}
	if got.Utterances[0].Speaker != "Alice" {
		t.Errorf("utterance speaker not renamed: %q", got.Utterances[0].Speaker)
	}
	if got.Utterances[1].Speaker != "SPEAKER_02" {
		t.Errorf("unrelated speaker should be untouched: %q", got.Utterances[1].Speaker)
	}
	if got.Words[0].Speaker != "Alice" {
		t.Errorf("word speaker not renamed: %q", got.Words[0].Speaker)
	}
	if tr.Utterances[0].Speaker != "SPEAKER_01" {
		t.Error("input transcript should not be mutated")
	}
}

func TestItemStartTime_PrefersRecordedAt(t *testing.T) {
	when := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	item := ExportItem{RecordedAt: when, SourcePath: "/nonexistent/audio.wav"}
	got := itemStartTime(item)
	if !got.Equal(when) {
		t.Errorf("got %v, want %v", got, when)
	}
}

func TestItemStartTime_FallsBackToNow(t *testing.T) {
	item := ExportItem{SourcePath: "/nonexistent/audio.wav"}
	before := time.Now()
	got := itemStartTime(item)
	after := time.Now()
	if got.Before(before) || got.After(after) {
		t.Errorf("expected fallback to time.Now(), got %v (range %v–%v)", got, before, after)
	}
}
