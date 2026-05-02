//go:build !android

package transcribe

func syscallGettid() int { return 0 }
