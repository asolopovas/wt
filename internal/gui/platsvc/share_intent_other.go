//go:build !android

package platsvc

func ShareIntakeChan() <-chan string { return nil }

func PollShareIntent() {}
