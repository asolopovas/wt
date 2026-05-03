package platsvc

import "strings"

func sanitizeShareName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "share.bin"
	}
	var b strings.Builder
	b.Grow(len(name))
	for _, r := range name {
		switch r {
		case '/', '\\', ':', 0:
			b.WriteByte('_')
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}
