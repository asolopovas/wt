//go:build android

package platsvc

import "testing"

func TestSanitizeShareName(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", "share.bin"},
		{"whitespace_only", "   ", "share.bin"},
		{"simple", "transcript.txt", "transcript.txt"},
		{"unicode_kept", "résumé.txt", "résumé.txt"},
		{"unix_separator", "../etc/passwd", ".._etc_passwd"},
		{"windows_separator", `C:\Users\evil.exe`, "C__Users_evil.exe"},
		{"colon_only", "a:b:c", "a_b_c"},
		{"trim_then_sanitize", "  bad/name  ", "bad_name"},
		{"null_byte", "abc\x00def", "abc_def"},
		{"forward_and_back_mix", "a/b\\c", "a_b_c"},
		{"keeps_dashes_and_dots", "2025-05-02_12.30.txt", "2025-05-02_12.30.txt"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeShareName(tt.in)
			if got != tt.want {
				t.Fatalf("sanitizeShareName(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
