//go:build !windows

package llm

import "os/exec"

func hideLlamaWindow(_ *exec.Cmd) {}
