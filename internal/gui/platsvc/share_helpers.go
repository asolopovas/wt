package platsvc

import "strings"

// sanitizeShareName returns name with characters that would break
// FileProvider path resolution (path separators, drive letters, NULs)
// replaced by underscores. Empty input becomes "share.bin".
//
// Kept build-tag-neutral so it is unit-testable on Linux CI even though
// it is only invoked from the android build of share_send_android.go.
func sanitizeShareName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "share.bin"
	}
	var b strings.Builder
	b.Grow(len(name))
	for _, r := range name {
		switch {
		case r == '/' || r == '\\' || r == ':' || r == 0:
			b.WriteByte('_')
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}
