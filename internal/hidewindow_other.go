//go:build !windows

package shared

import "os/exec"

func HideWindow(_ *exec.Cmd) {}
