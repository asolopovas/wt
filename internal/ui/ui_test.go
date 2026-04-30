package ui

import (
	"strings"
	"testing"
)

func TestFormatETA(t *testing.T) {
	tests := []struct {
		name        string
		elapsedSec  float64
		pct         float64
		want        string
		shouldEmpty bool
	}{
		{"zero progress returns empty", 5, 0, "", true},
		{"negative progress returns empty", 5, -10, "", true},
		{"50%% halfway", 30, 50, "~00:00:30", false},
		{"10%% projects 9× remaining", 10, 10, "~00:01:30", false},
		{"already past total clamps to zero", 100, 99, "~00:00:01", false},
		{"hours scale", 1800, 25, "~01:30:00", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatETA(tt.elapsedSec, tt.pct)
			if tt.shouldEmpty {
				if got != "" {
					t.Errorf("expected empty, got %q", got)
				}
				return
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestProgressBar(t *testing.T) {
	tests := []struct {
		pct, width  int
		filledRunes int
	}{
		{0, 10, 0},
		{50, 10, 5},
		{100, 10, 10},
		{150, 10, 10},
		{37, 30, 11},
	}
	for _, tt := range tests {
		got := progressBar(tt.pct, tt.width)
		gotRunes := []rune(got)
		if len(gotRunes) != tt.width {
			t.Errorf("pct=%d width=%d: rune len=%d want %d", tt.pct, tt.width, len(gotRunes), tt.width)
		}
		filled := strings.Count(got, "█")
		if filled != tt.filledRunes {
			t.Errorf("pct=%d width=%d: filled=%d want %d", tt.pct, tt.width, filled, tt.filledRunes)
		}
		empty := strings.Count(got, "░")
		if filled+empty != tt.width {
			t.Errorf("pct=%d width=%d: filled+empty=%d want %d", tt.pct, tt.width, filled+empty, tt.width)
		}
	}
}
