package transcriber

import (
	"testing"

	shared "github.com/asolopovas/wt/internal"
)

func TestResolveEngine(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty defaults to whisper", "", shared.EngineWhisper},
		{"explicit whisper", "whisper", shared.EngineWhisper},
		{"explicit zipformer", "zipformer", shared.EngineZipformer},
		{"unknown passes through for dispatcher to reject", "bogus", "bogus"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveEngine(tc.in)
			if got != tc.want {
				t.Fatalf("resolveEngine(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
