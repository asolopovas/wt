package gui

import (
	"strings"
	"testing"
	"time"
)

func TestFormatRelative(t *testing.T) {
	now := time.Now()
	tests := []struct {
		t       time.Time
		wantSub string
	}{
		{now.Add(-30 * time.Second), "just now"},
		{now.Add(-5 * time.Minute), "m ago"},
		{now.Add(-3 * time.Hour), "h ago"},
		{now.Add(-2 * 24 * time.Hour), "d ago"},
	}
	for _, tt := range tests {
		got := formatRelative(tt.t)
		if !strings.Contains(got, tt.wantSub) {
			t.Errorf("formatRelative(%v)=%q, want substring %q", tt.t, got, tt.wantSub)
		}
	}

	old := now.Add(-30 * 24 * time.Hour)
	got := formatRelative(old)
	if _, err := time.Parse("2006-01-02", got); err != nil {
		t.Errorf("expected ISO date for old time, got %q", got)
	}
}
