package diarizer

import "testing"

func TestSherpaSegRE_ParsesSegmentLine(t *testing.T) {
	tests := []struct {
		line              string
		match             bool
		start, end, spkID string
	}{
		{"  1.234 -- 5.678 speaker_3 ", true, "1.234", "5.678", "3"},
		{"0.00 -- 0.50 speaker_0", true, "0.00", "0.50", "0"},
		{"1 -- 2 speaker_1", false, "", "", ""},
		{"1.0 - 2.0 speaker_1", false, "", "", ""},
		{"random log line", false, "", "", ""},
	}
	for _, tt := range tests {
		m := sherpaSegRE.FindStringSubmatch(tt.line)
		if (m != nil) != tt.match {
			t.Errorf("line=%q matched=%v want=%v", tt.line, m != nil, tt.match)
			continue
		}
		if !tt.match {
			continue
		}
		if m[1] != tt.start || m[2] != tt.end || m[3] != tt.spkID {
			t.Errorf("line=%q got=[%s,%s,%s] want=[%s,%s,%s]",
				tt.line, m[1], m[2], m[3], tt.start, tt.end, tt.spkID)
		}
	}
}

func TestSherpaProgRE_ParsesProgress(t *testing.T) {
	tests := []struct {
		line  string
		match bool
		pct   string
	}{
		{"progress 12.5%", true, "12.5"},
		{"diarization progress 100.00% done", true, "100.00"},
		{"progress 50%", false, ""},
		{"no progress here", false, ""},
	}
	for _, tt := range tests {
		m := sherpaProgRE.FindStringSubmatch(tt.line)
		if (m != nil) != tt.match {
			t.Errorf("line=%q matched=%v want=%v", tt.line, m != nil, tt.match)
			continue
		}
		if tt.match && m[1] != tt.pct {
			t.Errorf("line=%q got=%q want=%q", tt.line, m[1], tt.pct)
		}
	}
}

func TestSherpaBinName_PerOS(t *testing.T) {
	got := sherpaBinName()
	if got == "" {
		t.Fatal("empty bin name")
	}
}
