package main

import (
	"net/http"
	"os/exec"
	"runtime"
)

// handleScreensaver starts the screensaver. Requires Enable Remote Screensaver to be enabled.
// macOS: open -a ScreenSaverEngine
// Windows: run default .scr (scrnsave.scr /s)
// Linux: xdg-screensaver activate (fallback: xscreensaver-command -activate)
func handleScreensaver(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")

	if !getEnableRemoteScreensaver() {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"success":false,"error":"Remote screensaver is disabled in client settings"}`))
		return
	}

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", "-a", "ScreenSaverEngine")
	case "windows":
		// scrnsave.scr /s starts the default screensaver in fullscreen
		cmd = exec.Command("cmd", "/c", "C:\\Windows\\System32\\scrnsave.scr", "/s")
		prepareCmd(cmd)
	case "linux":
		// xdg-screensaver is desktop-agnostic (GNOME, KDE, etc.)
		cmd = exec.Command("xdg-screensaver", "activate")
		if err := cmd.Run(); err != nil {
			// Fallback: xscreensaver
			cmd = exec.Command("xscreensaver-command", "-activate")
			if err := cmd.Run(); err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(`{"success":false,"error":"xdg-screensaver and xscreensaver failed"}`))
				return
			}
		}
	default:
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotImplemented)
		w.Write([]byte(`{"success":false,"error":"Screensaver not supported on this OS"}`))
		return
	}

	if runtime.GOOS == "darwin" || runtime.GOOS == "windows" {
		if err := cmd.Start(); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"success":false,"error":"` + err.Error() + `"}`))
			return
		}
		go func() { _ = cmd.Wait() }()
	}
	// Linux: already ran in the switch above

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"success":true,"message":"Screensaver started"}`))
}
