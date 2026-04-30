package gui

import "runtime"

var validModels = func() []string {
	m := []string{"tiny", "base", "small", "medium"}
	if runtime.GOOS != "android" {
		m = append(m, "large-v3")
	}
	return append(m, "turbo")
}()
