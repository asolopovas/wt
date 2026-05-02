//go:build !android

package platsvc

func QueryAudioFilesIn(prefix string) []string { return nil }
func RescanMediaPath(path string)              {}
func RescanMediaDir(dir string)                {}
