package transcriber

import (
	"testing"
	"time"
)

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
