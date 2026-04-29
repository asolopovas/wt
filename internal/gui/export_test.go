package gui

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
