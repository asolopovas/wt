package progress

import (
	"math"
	"testing"
)

func approxEq(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestDefaultRTF_ModelMatrix(t *testing.T) {
	tests := []struct {
		model, device string
		gpu, cpu      float64
	}{
		{"tiny.en", "cuda", 3, 1},
		{"base", "cuda", 2, 2.0 / 3},
		{"small", "cuda", 1, 1.0 / 3},
		{"medium.en", "cuda", 0.5, 0.5 / 3},
		{"large-v3", "cuda", 0.3, 0.3 / 3},
		{"large-v3-turbo", "cuda", 0.8, 0.8 / 3},
		{"unknown", "cuda", 1, 1.0 / 3},
	}

	for _, tt := range tests {
		got := DefaultRTF(tt.model, tt.device)
		if !approxEq(got, tt.gpu) {
			t.Errorf("DefaultRTF(%q, %q) = %v, want %v", tt.model, tt.device, got, tt.gpu)
		}
		got = DefaultRTF(tt.model, "cpu")
		if !approxEq(got, tt.cpu) {
			t.Errorf("DefaultRTF(%q, cpu) = %v, want %v", tt.model, got, tt.cpu)
		}
	}
}

func TestDefaultRTF_EmptyDeviceTreatedAsCPU(t *testing.T) {
	gpu := DefaultRTF("tiny", "cuda")
	empty := DefaultRTF("tiny", "")
	if empty >= gpu {
		t.Errorf("empty device should treat as CPU (slower); got empty=%v gpu=%v", empty, gpu)
	}
}

func TestDefaultRTF_LowerBound(t *testing.T) {
	if got := DefaultRTF("large", "cpu"); got < 0.05 {
		t.Errorf("RTF clamped below 0.05: got %v", got)
	}
}

func TestDefaultRTF_CaseInsensitive(t *testing.T) {
	if DefaultRTF("TINY", "CUDA") != DefaultRTF("tiny", "cuda") {
		t.Error("model+device matching should be case-insensitive")
	}
}
