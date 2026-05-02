package llm

import (
	"testing"
	"time"
)

func TestComputeLLMTimeout(t *testing.T) {
	tests := []struct {
		name string
		goos string
		env  string
		want time.Duration
	}{
		{"linux_default", "linux", "", 2 * time.Minute},
		{"darwin_default", "darwin", "", 2 * time.Minute},
		{"windows_default", "windows", "", 2 * time.Minute},
		{"android_default", "android", "", 10 * time.Minute},
		{"valid_override_900", "android", "900", 15 * time.Minute},
		{"valid_override_60", "linux", "60", 60 * time.Second},
		{"zero_override_ignored", "android", "0", 10 * time.Minute},
		{"negative_override_ignored", "android", "-5", 10 * time.Minute},
		{"non_numeric_ignored", "linux", "soon", 2 * time.Minute},
		{"empty_uses_default", "linux", "", 2 * time.Minute},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := computeLLMTimeout(tt.goos, tt.env)
			if got != tt.want {
				t.Fatalf("computeLLMTimeout(%q, %q) = %s, want %s", tt.goos, tt.env, got, tt.want)
			}
		})
	}
}

func TestLastBalancedJSON(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"single_object", `{"topic":"abc"}`, `{"topic":"abc"}`},
		{"with_prose_before", `Here you go: {"topic":"abc"}`, `{"topic":"abc"}`},
		{"with_prose_after", `{"topic":"abc"}\n\nDone.`, `{"topic":"abc"}`},
		{"prefers_last", `{"topic":"first"} ... {"topic":"last"}`, `{"topic":"last"}`},
		{"nested", `prose {"a":{"b":1}} tail`, `{"a":{"b":1}}`},
		{"unbalanced_returns_empty", `{"topic":"abc"`, ``},
		{"no_braces", `nothing here`, ``},
		{"empty", ``, ``},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := lastBalancedJSON(tt.in)
			if got != tt.want {
				t.Fatalf("lastBalancedJSON(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestStdoutTail(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		n       int
		wantStr string
	}{
		{"shorter_than_n", "hello", 100, "hello"},
		{"exact_n", "hello", 5, "hello"},
		{"longer_than_n", "1234567890", 4, "...7890"},
		{"trims_whitespace", "  hello  ", 100, "hello"},
		{"empty", "", 10, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := stdoutTail(tt.in, tt.n); got != tt.wantStr {
				t.Fatalf("stdoutTail(%q, %d) = %q, want %q", tt.in, tt.n, got, tt.wantStr)
			}
		})
	}
}

func TestStderrTail(t *testing.T) {
	tests := []struct {
		name string
		in   string
		n    int
		want string
	}{
		{"under_n_lines", "a\nb\nc", 5, "a\nb\nc"},
		{"exact_n", "a\nb\nc", 3, "a\nb\nc"},
		{"keeps_last_n", "a\nb\nc\nd\ne", 2, "d\ne"},
		{"strips_trailing_newline", "x\ny\n", 5, "x\ny"},
		{"empty", "", 5, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := stderrTail(tt.in, tt.n); got != tt.want {
				t.Fatalf("stderrTail(%q, %d) = %q, want %q", tt.in, tt.n, got, tt.want)
			}
		})
	}
}
