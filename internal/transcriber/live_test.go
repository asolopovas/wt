package transcriber

import (
	"encoding/binary"
	"math"
	"testing"
)

func TestPcmToFloat32(t *testing.T) {
	numSamples := 4
	buf := make([]byte, numSamples*2)

	binary.LittleEndian.PutUint16(buf[0:], 0)
	binary.LittleEndian.PutUint16(buf[2:], uint16(math.MaxInt16))
	binary.LittleEndian.PutUint16(buf[4:], 0x8000)
	binary.LittleEndian.PutUint16(buf[6:], uint16(16383))

	samples := pcmToFloat32(buf, numSamples)

	if len(samples) != numSamples {
		t.Fatalf("expected %d samples, got %d", numSamples, len(samples))
	}

	if samples[0] != 0.0 {
		t.Errorf("sample[0] = %f, want 0.0", samples[0])
	}

	if samples[1] < 0.99 || samples[1] > 1.01 {
		t.Errorf("sample[1] = %f, want ~1.0", samples[1])
	}

	if samples[2] > -0.99 || samples[2] < -1.01 {
		t.Errorf("sample[2] = %f, want ~-1.0", samples[2])
	}

	if samples[3] < 0.49 || samples[3] > 0.51 {
		t.Errorf("sample[3] = %f, want ~0.5", samples[3])
	}
}

func TestPcmToFloat32_Empty(t *testing.T) {
	samples := pcmToFloat32([]byte{}, 0)
	if len(samples) != 0 {
		t.Errorf("expected empty slice, got %d samples", len(samples))
	}
}
