package transcribe

import "testing"

func TestExtensionSet(t *testing.T) {
	set := extensionSet([]string{".wav", ".mp3", ".flac"})
	if len(set) != 3 {
		t.Errorf("len=%d want 3", len(set))
	}
	if !set[".wav"] || !set[".mp3"] || !set[".flac"] {
		t.Errorf("missing entries: %+v", set)
	}
	if set[".ogg"] {
		t.Error(".ogg should not be in set")
	}

	empty := extensionSet(nil)
	if len(empty) != 0 {
		t.Errorf("nil input: len=%d want 0", len(empty))
	}
}

func TestFormatETA(t *testing.T) {
	tests := []struct {
		secs float64
		want string
	}{
		{0, "0:00"},
		{59, "0:59"},
		{60, "1:00"},
		{125, "2:05"},
		{3600, "1:00:00"},
		{3661, "1:01:01"},
		{-5, "0:00"},
		{59.4, "0:59"},
	}
	for _, tt := range tests {
		got := formatETA(tt.secs)
		if got != tt.want {
			t.Errorf("formatETA(%v) = %q, want %q", tt.secs, got, tt.want)
		}
	}
}

func TestPanelDisplayName(t *testing.T) {
	p := &Panel{}
	if got := p.displayName("SPEAKER_01"); got != "SPEAKER_01" {
		t.Errorf("no rename: got %q, want %q", got, "SPEAKER_01")
	}

	p.speakerRenames = map[string]string{"SPEAKER_01": "Alice"}
	if got := p.displayName("SPEAKER_01"); got != "Alice" {
		t.Errorf("with rename: got %q, want %q", got, "Alice")
	}

	p.speakerRenames["SPEAKER_02"] = "   "
	if got := p.displayName("SPEAKER_02"); got != "SPEAKER_02" {
		t.Errorf("whitespace rename should fall back: got %q, want %q", got, "SPEAKER_02")
	}

	p.speakerRenames["SPEAKER_03"] = "  Bob  "
	if got := p.displayName("SPEAKER_03"); got != "Bob" {
		t.Errorf("padded rename should trim: got %q, want %q", got, "Bob")
	}
}
