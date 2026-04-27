//go:build darwin

package main

import (
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

const displayStateCacheTTL = 3 * time.Second

var (
	displayStateMu       sync.Mutex
	displayStateCache    DisplayStateInfo
	displayStateCachedAt time.Time
)

// getDisplayStateInfo returns detailed display state for debug.
// Apple Silicon: ioreg often omits IOPowerManagement; system_profiler may omit
// "Display Asleep" lines for built-in GPU-only trees — in that case we do not
// call pmset (health checks and volume polling would spawn it constantly).
func getDisplayStateInfo() DisplayStateInfo {
	displayStateMu.Lock()
	if time.Since(displayStateCachedAt) < displayStateCacheTTL && !displayStateCachedAt.IsZero() {
		out := displayStateCache
		displayStateMu.Unlock()
		return out
	}
	displayStateMu.Unlock()

	info := computeDisplayStateInfo()

	displayStateMu.Lock()
	displayStateCache = info
	displayStateCachedAt = time.Now()
	displayStateMu.Unlock()
	return info
}

func computeDisplayStateInfo() DisplayStateInfo {
	info := DisplayStateInfo{}
	if runtime.GOOS != "darwin" {
		return info
	}

	// 1. system_profiler SPDisplaysDataType
	if out, err := exec.Command("system_profiler", "SPDisplaysDataType").Output(); err == nil {
		s := string(out)
		countAsleep := strings.Count(s, "Display Asleep: Yes")
		countAwake := strings.Count(s, "Display Asleep: No")
		info.DisplayAsleepCount = countAsleep
		info.DisplayAwakeCount = countAwake
		if countAsleep > 0 && countAwake == 0 {
			info.IsDisplayAsleep = true
			info.CurrentPowerState = 1
			info.PowerStateSource = "system_profiler"
			info.IsDarkWake = true
			return info
		}
		if countAwake > 0 {
			info.IsDisplayAsleep = false
			info.CurrentPowerState = 4
			info.PowerStateSource = "system_profiler"
			info.IsDarkWake = false
			return info
		}
	}

	// 2. ioreg fallback (Intel; some configs still expose power state)
	if out, err := exec.Command("ioreg", "-n", "IODisplayWrangler", "-r", "-d", "6").Output(); err == nil {
		s := string(out)
		re := regexp.MustCompile(`"(?:CurrentPowerState|DevicePowerState|IOPowerState)"\s*=\s*(\d+)`)
		for _, m := range re.FindAllStringSubmatch(s, -1) {
			if len(m) >= 2 {
				if n, _ := strconv.Atoi(m[1]); n > 0 {
					info.CurrentPowerState = n
					info.PowerStateSource = "ioreg"
					info.IsDisplayAsleep = n < 4
					info.IsDarkWake = n < 4
					return info
				}
			}
		}
	}

	// Unknown: treat as full wake. Former pmset -g log fallback caused repeated
	// heavy subprocesses on every /health, heartbeat, and volume poll.
	info.IsDarkWake = false
	return info
}

// isDisplayInDarkWake returns true if the display is off (Power Nap / Dark Wake).
func isDisplayInDarkWake() bool {
	return getDisplayStateInfo().IsDarkWake
}
