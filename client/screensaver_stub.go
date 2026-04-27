//go:build !windows

package main

import "os/exec"

// startWindowsScreensaver is only used on Windows; this stub satisfies the build.
func startWindowsScreensaver() *exec.Cmd {
	return nil
}
