package progress

import (
	"testing"
	"time"
)

func TestSmoother_DefaultsClampedPositive(t *testing.T) {
	s := NewSmoother(0, 0)
	if s.audioDurSec <= 0 || s.rtf <= 0 {
		t.Fatalf("expected positive defaults, got dur=%v rtf=%v", s.audioDurSec, s.rtf)
	}
}

func TestSmoother_ReportIgnoresNonMonotonic(t *testing.T) {
	s := NewSmoother(60, 1)
	s.Report(20)
	s.Report(10)
	s.Report(20)
	if s.lastPct != 20 {
		t.Errorf("lastPct=%d want 20", s.lastPct)
	}
}

func TestSmoother_ObservedRTFRequiresTwoSamples(t *testing.T) {
	s := NewSmoother(60, 1)
	if got := s.ObservedRTF(); got != 0 {
		t.Errorf("with no samples ObservedRTF=%v want 0", got)
	}
	time.Sleep(10 * time.Millisecond)
	s.Report(10)
	time.Sleep(10 * time.Millisecond)
	s.Report(20)
	time.Sleep(10 * time.Millisecond)
	s.Report(30)
	if got := s.ObservedRTF(); got <= 0 {
		t.Errorf("after multiple samples ObservedRTF=%v want >0", got)
	}
}

func TestSmoother_SnapshotMonotonic(t *testing.T) {
	s := NewSmoother(60, 1)
	s.Report(50)

	prev, _ := s.Snapshot()
	for range 10 {
		time.Sleep(2 * time.Millisecond)
		d, _ := s.Snapshot()
		if d < prev {
			t.Fatalf("display went backwards: %v -> %v", prev, d)
		}
		prev = d
	}
}

func TestSmoother_SnapshotCappedBelow100(t *testing.T) {
	s := NewSmoother(1, 0.01)
	s.Report(95)
	time.Sleep(20 * time.Millisecond)
	d, _ := s.Snapshot()
	if d > 99 {
		t.Errorf("display=%v should be capped at 99", d)
	}
}

func TestSmoother_ETANonNegative(t *testing.T) {
	s := NewSmoother(60, 1)
	s.Report(99)
	_, eta := s.Snapshot()
	if eta < 0 {
		t.Errorf("eta negative: %v", eta)
	}
}
