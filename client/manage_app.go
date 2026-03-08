package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v4/process"
)

// handleManageApp handles GET /manage-app?name=X&target=on|off
// target=on: launch app (macOS: open -a "AppName", Windows: start "" "AppName" or similar)
// target=off: kill all processes matching name (case-insensitive, .exe handling on Windows)
func handleManageApp(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	name := strings.TrimSpace(r.URL.Query().Get("name"))
	target := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("target")))

	if name == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Missing name parameter"})
		return
	}
	if target != "on" && target != "off" {
		target = "on"
	}

	w.Header().Set("Content-Type", "application/json")

	if target == "on" {
		if err := launchApp(name); err != nil {
			log.Printf("⚠️  Launch app %q failed: %v", name, err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": err.Error()})
			return
		}
		log.Printf("🚀 Launched app: %s", name)
		json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "message": "App launched"})
		return
	}

	// target=off: kill processes
	killed, err := killProcessesByName(name)
	if err != nil {
		log.Printf("⚠️  Kill processes %q failed: %v", name, err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": err.Error()})
		return
	}
	log.Printf("🛑 Killed %d process(es) matching %q", killed, name)
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "message": "Processes killed", "count": killed})

	// Sleep after quit: same as Sleep After Action (5s delay)
	if app := getManagedAppByName(name); app != nil && app.SleepAfter {
		go func() {
			time.Sleep(5 * time.Second)
			triggerSleep()
		}()
	}
}

// launchApp launches an application by name.
// macOS: open -a "AppName"
// Windows: start "" "AppName" or start "" "AppName.exe"
// Linux: try xdg-open or similar
func launchApp(name string) error {
	switch runtime.GOOS {
	case "darwin":
		// macOS: open -a "Safari" (clean name)
		cmd := exec.Command("open", "-a", name)
		return cmd.Start()
	case "windows":
		// Windows: start "" "AppName" — pass name as-is (user can include .exe)
		cmd := exec.Command("cmd", "/c", "start", "", name)
		prepareCmd(cmd)
		return cmd.Start()
	default:
		// Linux: try running the name directly (e.g. firefox, chromium)
		cmd := exec.Command(name)
		return cmd.Start()
	}
}

// killProcessesByName kills all processes matching the given name (case-insensitive).
func killProcessesByName(name string) (int, error) {
	procs, err := process.Processes()
	if err != nil {
		return 0, err
	}
	killed := 0
	for _, p := range procs {
		procName, err := p.Name()
		if err != nil || procName == "" {
			continue
		}
		if !ProcessNameMatches(procName, name) {
			continue
		}
		if err := p.Kill(); err != nil {
			log.Printf("⚠️  Failed to kill PID %d (%s): %v", p.Pid, procName, err)
			continue
		}
		killed++
	}
	return killed, nil
}
