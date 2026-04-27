package main

import (
	"log"
	"net/http"
	"os/exec"
	"runtime"
)

// handleLock locks the screen. Does NOT put the computer to sleep.
// macOS: Cmd+Ctrl+Q (key code 12) via osascript
// Windows: rundll32 user32.dll,LockWorkStation
// Linux: xdg-screensaver lock (fallback: xscreensaver-command -lock)
func handleLock(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")

	if !getEnableRemoteLock() {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"success":false,"error":"Remote lock is disabled in client settings"}`))
		return
	}

	log.Printf("🔒 Lock request received")
	appendLog("🔒 Lock request received")

	if runtime.GOOS == "darwin" {
		handleLockDarwin(w, r)
		return
	}

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32.exe", "user32.dll,LockWorkStation")
		prepareCmd(cmd)
	case "linux":
		cmd = exec.Command("xdg-screensaver", "lock")
		if err := cmd.Run(); err != nil {
			cmd = exec.Command("xscreensaver-command", "-lock")
			if err := cmd.Run(); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(`{"success":false,"error":"xdg-screensaver and xscreensaver lock failed"}`))
				return
			}
		}
	default:
		w.WriteHeader(http.StatusNotImplemented)
		w.Write([]byte(`{"success":false,"error":"Lock not supported on this OS"}`))
		return
	}

	if runtime.GOOS == "linux" {
		// Already ran above
	} else {
		if err := cmd.Run(); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"success":false,"error":"` + err.Error() + `"}`))
			return
		}
	}

	w.Write([]byte(`{"success":true,"message":"Screen locked"}`))
}
