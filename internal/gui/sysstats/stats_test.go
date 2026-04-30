package sysstats

import "testing"

func TestFormatMB(t *testing.T) {
	tests := []struct {
		mb   int64
		want string
	}{
		{0, "0 MB"},
		{1, "1 MB"},
		{512, "512 MB"},
		{1023, "1023 MB"},
		{1024, "1.0 GB"},
		{1536, "1.5 GB"},
		{16384, "16.0 GB"},
	}
	for _, tt := range tests {
		got := FormatMB(tt.mb)
		if got != tt.want {
			t.Errorf("FormatMB(%d) = %q, want %q", tt.mb, got, tt.want)
		}
	}
}
