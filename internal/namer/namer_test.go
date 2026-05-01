package namer

import (
	"strings"
	"testing"
	"time"
)

func TestSanitizeTopic(t *testing.T) {
	cases := map[string]string{
		"  Quarterly Review ":    "quarterly-review",
		"foo/bar baz":            "foo-bar-baz",
		"hello___world":          "hello-world",
		"---trim me---":          "trim-me",
		"emoji 🎉 ok":             "emoji-ok",
		strings.Repeat("a", 80): strings.Repeat("a", 60),
	}
	for in, want := range cases {
		got := sanitizeTopic(in)
		if got != want {
			t.Errorf("sanitizeTopic(%q): got %q want %q", in, got, want)
		}
	}
}

func TestFilename(t *testing.T) {
	fb := time.Date(2026, 4, 30, 13, 45, 5, 0, time.UTC)
	s := Suggestion{Stamp: fb.Format("060102-150405"), Topic: "team-sync"}
	if got := s.Filename(".m4a"); got != "260430-134505_team-sync.m4a" {
		t.Errorf("filename: got %s", got)
	}
	if got := s.Filename(""); got != "260430-134505_team-sync" {
		t.Errorf("filename no ext: got %s", got)
	}
}

func TestExtractJSON_Utterances(t *testing.T) {
	data := []byte(`{"utterances":[{"speaker":"S1","text":"Hello world."},{"speaker":"S2","text":"Reply."}]}`)
	out, err := extractJSON(data)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "S1: Hello world.") || !strings.Contains(out, "S2: Reply.") {
		t.Errorf("extracted text missing utterances: %q", out)
	}
}
