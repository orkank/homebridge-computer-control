package main

import (
	"os/exec"
	"runtime"
	"sort"
	"strings"

	"github.com/shirou/gopsutil/v4/process"
)

// systemProcessPrefixes are process names we exclude (kernel/system).
var systemProcessPrefixes = []string{
	"kernel_task", "launchd", "WindowServer", "loginwindow",
	"systemd", "init", "kthreadd", "ksoftirqd", "kworker",
	"migration", "rcu_sched", "watchdog", "cpuhp",
	"idle", "[", "(", "sudo", "su ",
}

// isSystemProcess returns true if the process should be excluded (kernel/system).
func isSystemProcess(name string) bool {
	n := strings.TrimSpace(strings.ToLower(name))
	if n == "" {
		return true
	}
	for _, p := range systemProcessPrefixes {
		if strings.HasPrefix(n, strings.ToLower(p)) || n == strings.ToLower(p) {
			return true
		}
	}
	// Exclude very short names (often system)
	if len(n) < 2 {
		return true
	}
	return false
}

// extractMacAppName converts "Safari.app/Contents/MacOS/Safari" → "Safari".
func extractMacAppName(comm string) string {
	// Handle .app path: AppName.app/Contents/MacOS/AppName
	if idx := strings.Index(comm, ".app/"); idx > 0 {
		return comm[:idx]
	}
	// Handle .app at end
	if strings.HasSuffix(comm, ".app") {
		return strings.TrimSuffix(comm, ".app")
	}
	return comm
}

// GetRunningProcessNames returns user application names (clean, deduplicated).
// macOS: clean names from .app (e.g. "Safari" not "Safari.app/Contents/MacOS/Safari")
// Windows: include .exe (e.g. "chrome.exe")
// Linux: process names as-is
func GetRunningProcessNames() []string {
	seen := make(map[string]bool)
	var names []string

	procs, err := process.Processes()
	if err != nil {
		// Fallback to ps on Unix
		return getRunningProcessNamesPS()
	}

	for _, p := range procs {
		name, err := p.Name()
		if err != nil || name == "" {
			continue
		}
		name = strings.TrimSpace(name)
		if isSystemProcess(name) {
			continue
		}

		var display string
		switch runtime.GOOS {
		case "darwin":
			display = extractMacAppName(name)
		case "windows":
			// Windows: keep .exe
			display = name
		default:
			display = name
		}

		display = strings.TrimSpace(display)
		if display == "" || isSystemProcess(display) {
			continue
		}
		key := strings.ToLower(display)
		if !seen[key] {
			seen[key] = true
			names = append(names, display)
		}
	}

	sort.Strings(names)
	return names
}

// getRunningProcessNamesPS fallback using ps when gopsutil fails.
func getRunningProcessNamesPS() []string {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin", "linux":
		cmd = exec.Command("ps", "-eo", "comm=")
	case "windows":
		cmd = exec.Command("tasklist", "/FO", "CSV", "/NH")
		prepareCmd(cmd)
	default:
		return nil
	}
	out, err := cmd.Output()
	if err != nil {
		return nil
	}

	seen := make(map[string]bool)
	var names []string
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var name string
		if runtime.GOOS == "windows" {
			// tasklist CSV: "name","pid","session","mem","status"
			parts := strings.SplitN(line, ",", 2)
			if len(parts) >= 1 {
				name = strings.Trim(parts[0], `"`)
			}
		} else {
			// ps -eo comm= gives just the command name
			name = line
		}
		name = strings.TrimSpace(name)
		if isSystemProcess(name) {
			continue
		}
		var display string
		switch runtime.GOOS {
		case "darwin":
			display = extractMacAppName(name)
		case "windows":
			display = name
		default:
			display = name
		}
		display = strings.TrimSpace(display)
		if display == "" || isSystemProcess(display) {
			continue
		}
		key := strings.ToLower(display)
		if !seen[key] {
			seen[key] = true
			names = append(names, display)
		}
	}
	sort.Strings(names)
	return names
}

// ProcessNameMatches returns true if the process name matches the target (case-insensitive).
// Windows: strip/add .exe as needed. macOS: extract from .app path.
func ProcessNameMatches(procName, target string) bool {
	p := strings.TrimSpace(strings.ToLower(procName))
	t := strings.TrimSpace(strings.ToLower(target))
	if p == t {
		return true
	}
	if runtime.GOOS == "windows" {
		tNoExe := strings.TrimSuffix(t, ".exe")
		pNoExe := strings.TrimSuffix(p, ".exe")
		if pNoExe == tNoExe {
			return true
		}
		if p == tNoExe+".exe" || pNoExe == t {
			return true
		}
	}
	if runtime.GOOS == "darwin" {
		clean := extractMacAppName(procName)
		if strings.ToLower(clean) == t {
			return true
		}
	}
	return false
}

// IsProcessRunning returns true if any process matches the given name (case-insensitive).
func IsProcessRunning(name string) bool {
	procs, err := process.Processes()
	if err != nil {
		return false
	}
	for _, p := range procs {
		procName, err := p.Name()
		if err != nil || procName == "" {
			continue
		}
		if ProcessNameMatches(procName, name) {
			return true
		}
	}
	return false
}
