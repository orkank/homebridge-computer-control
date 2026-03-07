//go:build windows

package main

import (
	"os/exec"
	"syscall"
)

// prepareCmd hides the console window for subprocesses on Windows.
// Call this before cmd.Run(), cmd.Output(), or cmd.Start().
func prepareCmd(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.HideWindow = true
}
