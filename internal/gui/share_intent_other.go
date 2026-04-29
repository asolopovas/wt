//go:build !android

package gui

func shareIntakeChan() <-chan string { return nil }

func pollShareIntent() {}
