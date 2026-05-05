package namer

import (
	"strings"
	"testing"
	"time"
)

func TestExcerptLimit(t *testing.T) {
	tests := []struct {
		name string
		goos string
		env  string
		want int
	}{
		{"desktop_default", "linux", "", 6000},
		{"darwin_default", "darwin", "", 6000},
		{"windows_default", "windows", "", 6000},
		{"android_default", "android", "", 1500},
		{"valid_override", "linux", "3000", 3000},
		{"valid_override_android", "android", "4000", 4000},
		{"override_too_small_ignored", "linux", "100", 6000},
		{"override_at_boundary_ignored", "linux", "200", 6000},
		{"override_just_above_boundary", "linux", "201", 201},
		{"non_numeric_override_ignored", "android", "abc", 1500},
		{"negative_override_ignored", "linux", "-500", 6000},
		{"empty_env_uses_default", "android", "", 1500},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := excerptLimit(tt.goos, tt.env); got != tt.want {
				t.Fatalf("excerptLimit(%q, %q) = %d, want %d", tt.goos, tt.env, got, tt.want)
			}
		})
	}
}

func TestTruncateExcerpt(t *testing.T) {
	long := strings.Repeat("a", 10_000)
	tests := []struct {
		name    string
		in      string
		limit   int
		wantLen int
	}{
		{"under_limit", "hello", 100, 5},
		{"exactly_limit", "hello", 5, 5},
		{"over_limit", long, 1500, 1500},
		{"zero_limit_returns_input", "hello", 0, 5},
		{"negative_limit_returns_input", "hello", -1, 5},
		{"empty_input", "", 100, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateExcerpt(tt.in, tt.limit)
			if len(got) != tt.wantLen {
				t.Fatalf("len(truncateExcerpt(%d chars, limit=%d)) = %d, want %d",
					len(tt.in), tt.limit, len(got), tt.wantLen)
			}
		})
	}
}

func TestSanitizeTopic(t *testing.T) {
	tests := []struct {
		name, in, want string
	}{
		{"already_clean", "kitchen-renovation-quote", "kitchen-renovation-quote"},
		{"uppercase_lowered", "KITCHEN-Quote", "kitchen-quote"},
		{"spaces_to_dashes", "weekly sales meeting", "weekly-sales-meeting"},
		{"strips_punctuation", "ai/ml: deep-dive!", "ai-ml-deep-dive"},
		{"collapses_runs", "foo---bar", "foo-bar"},
		{"trims_edge_dashes", "---hello---", "hello"},
		{"truncates_to_60", strings.Repeat("a", 100), strings.Repeat("a", 60)},
		{"trim_dash_after_truncate", strings.Repeat("a", 59) + "-extra", strings.Repeat("a", 59)},
		{"empty_after_strip", "!!!", ""},
		{"unicode_dropped", "café-meeting", "caf-meeting"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sanitizeTopic(tt.in); got != tt.want {
				t.Fatalf("sanitizeTopic(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestSuggestionFilename(t *testing.T) {
	stamp := time.Date(2025, 5, 2, 19, 36, 47, 0, time.UTC).Format("060102-150405")
	tests := []struct {
		name string
		s    Suggestion
		ext  string
		want string
	}{
		{"with_ext", Suggestion{Stamp: stamp, Topic: "kitchen-quote"}, "m4a", "kitchen-quote_" + stamp + ".m4a"},
		{"ext_with_dot", Suggestion{Stamp: stamp, Topic: "kitchen-quote"}, ".m4a", "kitchen-quote_" + stamp + ".m4a"},
		{"no_ext", Suggestion{Stamp: stamp, Topic: "kitchen-quote"}, "", "kitchen-quote_" + stamp},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.s.Filename(tt.ext); got != tt.want {
				t.Fatalf("Filename(%q) = %q, want %q", tt.ext, got, tt.want)
			}
		})
	}
}
