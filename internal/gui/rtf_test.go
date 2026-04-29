package gui

import (
	"testing"

	"github.com/asolopovas/wt/internal/progress"
)

func TestRtfKey_CaseInsensitive(t *testing.T) {
	if rtfKey("Tiny", "CUDA") != rtfKey("tiny", "cuda") {
		t.Error("rtfKey should be case-insensitive")
	}
}

func TestLoadRTF_MissingFileFallsBackToDefault(t *testing.T) {
	redirectAppDir(t)
	got := loadRTF("tiny", "cuda")
	want := progress.DefaultRTF("tiny", "cuda")
	if got != want {
		t.Errorf("got=%v want default %v", got, want)
	}
}

func TestSaveLoadRTF_RoundTrip(t *testing.T) {
	redirectAppDir(t)
	saveRTF("tiny", "cuda", 2.5)
	if got := loadRTF("tiny", "cuda"); got != 2.5 {
		t.Errorf("first save: got=%v want 2.5", got)
	}
	saveRTF("tiny", "cuda", 4.5)
	if got := loadRTF("tiny", "cuda"); got != 3.5 {
		t.Errorf("EMA blend: got=%v want 3.5 (0.5*2.5+0.5*4.5)", got)
	}
}

func TestSaveRTF_IgnoresNonPositive(t *testing.T) {
	redirectAppDir(t)
	saveRTF("tiny", "cuda", 0)
	saveRTF("tiny", "cuda", -1)
	want := progress.DefaultRTF("tiny", "cuda")
	if got := loadRTF("tiny", "cuda"); got != want {
		t.Errorf("got=%v want default %v", got, want)
	}
}
