//go:build darwin

package main

import (
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
)

// getDisplayStateInfo returns detailed display state for debug.
// Apple Silicon: ioreg no longer has IOPowerManagement; use system_profiler instead.
func getDisplayStateInfo() DisplayStateInfo {
	info := DisplayStateInfo{}
	if runtime.GOOS != "darwin" {
		return info
	}
	// 1. system_profiler SPDisplaysDataType — works on Apple Silicon (ioreg power state removed)
	if out, err := exec.Command("system_profiler", "SPDisplaysDataType").Output(); err == nil {
		s := string(out)
		countAsleep := strings.Count(s, "Display Asleep: Yes")
		countAwake := strings.Count(s, "Display Asleep: No")
		info.DisplayAsleepCount = countAsleep
		info.DisplayAwakeCount = countAwake
		if countAsleep > 0 && countAwake == 0 {
			info.IsDisplayAsleep = true
			info.CurrentPowerState = 1 // asleep
		} else if countAwake > 0 {
			info.IsDisplayAsleep = false
			info.CurrentPowerState = 4 // awake
		}
	}
	// 2. ioreg fallback for Intel Macs (has IOPowerManagement)
	if info.CurrentPowerState == 0 {
		if out, err := exec.Command("ioreg", "-n", "IODisplayWrangler", "-r", "-d", "6").Output(); err == nil {
			s := string(out)
			re := regexp.MustCompile(`"(?:CurrentPowerState|DevicePowerState|IOPowerState)"\s*=\s*(\d+)`)
			for _, m := range re.FindAllStringSubmatch(s, -1) {
				if len(m) >= 2 {
					if n, _ := strconv.Atoi(m[1]); n > 0 {
						info.CurrentPowerState = n
						info.PowerStateSource = "ioreg"
						break
					}
				}
			}
		}
	} else {
		info.PowerStateSource = "system_profiler"
	}
	info.IsDarkWake = isDisplayInDarkWake()
	return info
}

// isDisplayInDarkWake returns true if the display is off (Power Nap / Dark Wake).
// Uses system_profiler on Apple Silicon (ioreg power state removed by Apple).
func isDisplayInDarkWake() bool {
	if runtime.GOOS != "darwin" {
		return false
	}
	// system_profiler: "Display Asleep: Yes" when display is off (works on Apple Silicon)
	if out, err := exec.Command("system_profiler", "SPDisplaysDataType").Output(); err == nil {
		s := string(out)
		countAsleep := strings.Count(s, "Display Asleep: Yes")
		countAwake := strings.Count(s, "Display Asleep: No")
		if countAsleep > 0 && countAwake == 0 {
			return true
		}
		if countAwake > 0 {
			return false
		}
	}
	// ioreg fallback for Intel
	if out, err := exec.Command("ioreg", "-n", "IODisplayWrangler", "-r", "-d", "6").Output(); err == nil {
		re := regexp.MustCompile(`"(?:CurrentPowerState|DevicePowerState|IOPowerState)"\s*=\s*(\d+)`)
		if m := re.FindStringSubmatch(string(out)); len(m) >= 2 {
			if n, err := strconv.Atoi(m[1]); err == nil && n > 0 {
				return n < 4
			}
		}
	}
	// pmset log fallback
	if out, err := exec.Command("pmset", "-g", "log").Output(); err == nil {
		lines := strings.Split(string(out), "\n")
		for i := len(lines) - 1; i >= 0; i-- {
			line := strings.TrimSpace(lines[i])
			if line == "" {
				continue
			}
			lower := strings.ToLower(line)
			if strings.Contains(lower, "wake") {
				return false
			}
			if strings.Contains(lower, "sleep") {
				return true
			}
		}
	}
	return false
}
