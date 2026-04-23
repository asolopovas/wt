package shared

import (
	"os/exec"
	"syscall"
)

const createNoWindow = 0x08000000

func HideWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: createNoWindow,
	}
}
