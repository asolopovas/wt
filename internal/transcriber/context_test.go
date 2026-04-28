package transcriber

import "testing"

func TestUseTDRZ_NoDiarizeDisablesFallback(t *testing.T) {
	if UseTDRZ(false, false, true) {
		t.Fatal("UseTDRZ returned true, want false")
	}
}

func TestUseTDRZ_EnablesFallbackWhenDiarizerUnavailable(t *testing.T) {
	if !UseTDRZ(false, false, false) {
		t.Fatal("UseTDRZ returned false, want true")
	}
}
