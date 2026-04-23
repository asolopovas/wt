package transcriber

import (
	"testing"
	"time"
)

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		input time.Duration
		want  string
	}{
		{0, "0.000"},
		{500 * time.Millisecond, "0.500"},
		{1 * time.Second, "1.000"},
		{1*time.Minute + 30*time.Second + 500*time.Millisecond, "90.500"},
		{2*time.Hour + 30*time.Minute, "9000.000"},
	}

	for _, tt := range tests {
		got := FormatDuration(tt.input)
		if got != tt.want {
			t.Errorf("FormatDuration(%v) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestFormatHMS(t *testing.T) {
	tests := []struct {
		input time.Duration
		want  string
	}{
		{0, "0:00"},
		{30 * time.Second, "0:30"},
		{1*time.Minute + 5*time.Second, "1:05"},
		{59*time.Minute + 59*time.Second, "59:59"},
		{1 * time.Hour, "1:00:00"},
		{1*time.Hour + 19*time.Minute + 18*time.Second, "1:19:18"},
		{2*time.Hour + 30*time.Minute + 45*time.Second, "2:30:45"},
	}

	for _, tt := range tests {
		got := FormatHMS(tt.input)
		if got != tt.want {
			t.Errorf("FormatHMS(%v) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
