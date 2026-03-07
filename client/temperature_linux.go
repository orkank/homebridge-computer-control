package main

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// getCPUTemperatureMillidegree returns CPU temperature in millidegrees Celsius, or 0 if unavailable.
// Only runs when Send Temperature Data is enabled (caller checks).
// Primary: /sys/class/thermal/thermal_zone*/temp (prefer x86_pkg_temp or cpu-thermal)
// Fallback: sensors command, parse "Package id 0" or "Core 0"
func getCPUTemperatureMillidegree() int {
	if v := getTemperatureFromSysfs(); v > 0 {
		return v
	}
	if v := getTemperatureFromSensors(); v > 0 {
		return v
	}
	log.Printf("🌡️ Temperature: linux read failed — sysfs and sensors returned no valid value")
	return 0
}

func getTemperatureFromSysfs() int {
	matches, err := filepath.Glob("/sys/class/thermal/thermal_zone*/temp")
	if err != nil {
		log.Printf("🌡️ Temperature: sysfs glob failed: %v", err)
		return 0
	}
	if len(matches) == 0 {
		log.Printf("🌡️ Temperature: no thermal_zone* found under /sys/class/thermal")
		return 0
	}

	// Prefer zones with type x86_pkg_temp or cpu-thermal
	type zone struct {
		temp int
		prio int // 2=preferred, 1=other thermal, 0=unknown
	}
	var best zone

	for _, tempPath := range matches {
		zoneDir := filepath.Dir(tempPath)
		typePath := filepath.Join(zoneDir, "type")

		data, err := os.ReadFile(tempPath)
		if err != nil {
			continue
		}
		milli, err := strconv.Atoi(strings.TrimSpace(string(data)))
		if err != nil || milli <= 0 || milli > 150000 {
			continue
		}

		prio := 0
		if typeData, err := os.ReadFile(typePath); err == nil {
			t := strings.TrimSpace(string(typeData))
			if t == "x86_pkg_temp" || t == "cpu-thermal" {
				prio = 2
			} else if strings.Contains(t, "cpu") || strings.Contains(t, "pkg") {
				prio = 1
			}
		}

		if prio > best.prio || (prio == best.prio && milli > best.temp) {
			best = zone{temp: milli, prio: prio}
		}
	}

	if best.temp == 0 {
		log.Printf("🌡️ Temperature: sysfs thermal zones found but no valid temp read")
	}
	return best.temp
}

func getTemperatureFromSensors() int {
	cmd := exec.Command("sensors")
	out, err := cmd.Output()
	if err != nil {
		log.Printf("🌡️ Temperature: sensors command failed (install lm-sensors?): %v", err)
		return 0
	}

	// Parse "Package id 0:  +45.0°C" or "Core 0:        +42.0°C"
	rePkg := regexp.MustCompile(`(?i)Package id 0:\s*\+\s*([\d.]+)`)
	reCore := regexp.MustCompile(`(?i)Core 0:\s*\+\s*([\d.]+)`)
	lines := strings.Split(string(out), "\n")

	for _, line := range lines {
		var match []string
		if m := rePkg.FindStringSubmatch(line); len(m) >= 2 {
			match = m
		} else if m := reCore.FindStringSubmatch(line); len(m) >= 2 {
			match = m
		}
		if len(match) < 2 {
			continue
		}
		f, err := strconv.ParseFloat(strings.TrimSpace(match[1]), 64)
		if err != nil || f < 10 || f > 120 {
			continue
		}
		return int(f * 1000) // Celsius to millidegree
	}
	log.Printf("🌡️ Temperature: sensors output has no Package id 0 or Core 0")
	return 0
}
