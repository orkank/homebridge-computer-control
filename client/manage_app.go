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
// target=off: quit or kill based on app's QuitMode (quit, kill, quit_then_kill)
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

	// target=off: quit or kill based on QuitMode
	app := getManagedAppByName(name)
	quitMode := QuitModeKill
	if app != nil && app.QuitMode != "" {
		quitMode = app.QuitMode
	}

	var count int
	var msg string
	var err error
	switch quitMode {
	case QuitModeQuit:
		count, err = quitApp(name)
		if err != nil {
			log.Printf("⚠️  Quit app %q failed: %v", name, err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": err.Error()})
			return
		}
		msg = "Quit request sent"
	case QuitModeQuitKill:
		count, err = quitThenKill(name)
		if err != nil {
			log.Printf("⚠️  Quit/kill app %q failed: %v", name, err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": err.Error()})
			return
		}
		msg = "Quit sent; killed if still running"
	default:
		count, err = killProcessesByName(name)
		if err != nil {
			log.Printf("⚠️  Kill processes %q failed: %v", name, err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": err.Error()})
			return
		}
		msg = "Processes killed"
	}

	log.Printf("🛑 %s: %d process(es) for %q", msg, count, name)
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "message": msg, "count": count})

	// Sleep after quit: same as Sleep After Action (5s delay)
	if app != nil && app.SleepAfter {
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

// quitApp sends a graceful quit request. Returns count of processes that received the signal.
// macOS: osascript quit app; Windows: taskkill without /F; Linux: SIGTERM.
func quitApp(name string) (int, error) {
	if !IsProcessRunning(name) {
		return 0, nil
	}
	switch runtime.GOOS {
	case "darwin":
		cmd := exec.Command("osascript", "-e", `quit app "`+escapeAppleScriptString(name)+`"`)
		if err := cmd.Run(); err != nil {
			return 0, err
		}
		return 1, nil
	case "windows":
		// taskkill without /F sends WM_CLOSE to GUI apps (graceful)
		exeName := name
		if !strings.HasSuffix(strings.ToLower(exeName), ".exe") {
			exeName = name + ".exe"
		}
		cmd := exec.Command("taskkill", "/IM", exeName)
		prepareCmd(cmd)
		if err := cmd.Run(); err != nil {
			// taskkill returns non-zero if process not found or refused
			return 0, err
		}
		return 1, nil
	default:
		// Linux: killall sends SIGTERM (graceful) to all processes matching name
		cmd := exec.Command("killall", name)
		if err := cmd.Run(); err != nil {
			// killall returns 1 if no match
			return 0, err
		}
		return 1, nil
	}
}

func escapeAppleScriptString(s string) string {
	return strings.ReplaceAll(s, `"`, `\"`)
}

// quitThenKill tries quit first, waits 4s, then kills if still running.
func quitThenKill(name string) (int, error) {
	quitApp(name)
	time.Sleep(4 * time.Second)
	if IsProcessRunning(name) {
		return killProcessesByName(name)
	}
	return 1, nil
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
