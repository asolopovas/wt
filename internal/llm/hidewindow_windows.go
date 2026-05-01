package llm

import (
	"os/exec"
	"syscall"
)

func hideLlamaWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow: true,
	}
}
