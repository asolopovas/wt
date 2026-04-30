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
		strings.Repeat("a", 60):  strings.Repeat("a", 40),
	}
	for in, want := range cases {
		got := sanitizeTopic(in)
		if got != want {
			t.Errorf("sanitizeTopic(%q): got %q want %q", in, got, want)
		}
	}
}

func TestNormalize_FallbackOnInvalid(t *testing.T) {
	fb := time.Date(2026, 4, 30, 13, 45, 0, 0, time.UTC)
	s := &Suggestion{Date: "garbage", Time: "x", Topic: "Hello World"}
	s.normalize(fb)
	if s.Date != "2026-04-30" {
		t.Errorf("date: got %s", s.Date)
	}
	if s.Time != "13-45" {
		t.Errorf("time: got %s", s.Time)
	}
	if s.Topic != "hello-world" {
		t.Errorf("topic: got %s", s.Topic)
	}
}

func TestFilename(t *testing.T) {
	s := Suggestion{Date: "2026-04-30", Time: "13-45", Topic: "team-sync"}
	if got := s.Filename(".m4a"); got != "2026-04-30_13-45_team-sync.m4a" {
		t.Errorf("filename: got %s", got)
	}
	if got := s.Filename(""); got != "2026-04-30_13-45_team-sync" {
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
