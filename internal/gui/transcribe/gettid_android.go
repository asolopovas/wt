//go:build android

package transcribe

import "syscall"

func syscallGettid() int { return syscall.Gettid() }
