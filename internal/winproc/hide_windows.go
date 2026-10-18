//go:build windows

package winproc

import (
	"os/exec"
	"syscall"
)

const createNoWindow = 0x08000000

// HideConsole prevents clamscan/clamd console windows from flashing on Windows GUI builds.
func HideConsole(cmd *exec.Cmd) {
	if cmd == nil {
		return
	}
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.HideWindow = true
	cmd.SysProcAttr.CreationFlags |= createNoWindow
}
