//go:build !android

package sysstats

func ReserveTopCores(reserve int) ([]byte, bool) { return nil, false }
func RestoreAffinity(saved []byte)                {}
