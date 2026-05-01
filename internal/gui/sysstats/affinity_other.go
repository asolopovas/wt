//go:build !android

package sysstats

func AffinityCPUCount() int { return -1 }
