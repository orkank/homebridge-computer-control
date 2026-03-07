package main

import (
	"os/exec"
	"runtime"
)

// openURLInBrowser opens the given URL in the user's default browser.
func openURLInBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
		prepareCmd(cmd)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}
