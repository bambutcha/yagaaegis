//go:build !windows

package winproc

import "os/exec"

func HideConsole(cmd *exec.Cmd) {}
