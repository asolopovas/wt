//go:build !android

package gui

//nolint:unused // Build-tag stubs; consumed by share_intent_android.go under //go:build android.
func shareIntakeChan() <-chan string { return nil }

//nolint:unused // Build-tag stubs; consumed by share_intent_android.go under //go:build android.
func pollShareIntent() {}
