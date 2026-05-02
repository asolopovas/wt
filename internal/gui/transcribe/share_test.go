package transcribe

import "testing"

func TestMimeForExt(t *testing.T) {
	tests := []struct {
		name string
		ext  string
		want string
	}{
		{"txt", "txt", "text/plain"},
		{"csv", "csv", "text/csv"},
		{"json", "json", "application/json"},
		{"xlsx", "xlsx", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"},
		{"zip", "zip", "application/zip"},
		{"unknown_ext", "rtf", "application/octet-stream"},
		{"empty_ext", "", "application/octet-stream"},
		{"with_leading_dot", ".csv", "text/csv"},
		{"uppercase", "TXT", "text/plain"},
		{"mixed_case", "Json", "application/json"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := mimeForExt(tt.ext); got != tt.want {
				t.Fatalf("mimeForExt(%q) = %q, want %q", tt.ext, got, tt.want)
			}
		})
	}
}
