package main

import (
	"log"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/shirou/gopsutil/v4/sensors"
)

// getCPUTemperatureMillidegree returns CPU temperature in millidegrees Celsius, or 0 if unavailable.
// Only runs when Send Temperature Data is enabled (caller checks).
// Uses SMC (Intel) / HID sensors (Apple Silicon) via gopsutil, no external binaries.
func getCPUTemperatureMillidegree() int {
	// Primary: gopsutil sensors — SMC on Intel, HID on Apple Silicon (Tp09, TG0P, TC0P, etc.)
	if v := getTemperatureFromGopsutil(); v > 0 {
		return v
	}
	// Fallback: ioreg IOHWSensor (Intel)
	if v := getTemperatureFromIoreg(); v > 0 {
		return v
	}
	// Fallback: system_profiler SPSensorsDataType
	if v := getTemperatureFromSystemProfiler(); v > 0 {
		return v
	}
	// Last resort: powermetrics thermal (may need sudo; try without)
	if v := getTemperatureFromPowermetrics(); v > 0 {
		return v
	}
	log.Printf("🌡️ Temperature: darwin read failed — gopsutil, ioreg, system_profiler, powermetrics all returned no value")
	return 0
}

// getTemperatureFromGopsutil uses gopsutil sensors (SMC on Intel, HID on Apple Silicon).
func getTemperatureFromGopsutil() int {
	temps, err := sensors.SensorsTemperatures()
	if err != nil {
		log.Printf("🌡️ Temperature: gopsutil sensors failed: %v", err)
		return 0
	}
	var maxTemp float64
	for _, t := range temps {
		if t.Temperature > 10 && t.Temperature < 120 && t.Temperature > maxTemp {
			maxTemp = t.Temperature
		}
	}
	if maxTemp > 0 {
		return int(maxTemp * 1000)
	}
	log.Printf("🌡️ Temperature: gopsutil returned %d sensors but none in valid range 10-120°C", len(temps))
	return 0
}

func getTemperatureFromIoreg() int {
	cmd := exec.Command("ioreg", "-r", "-c", "IOHWSensor", "-w", "0", "-l")
	out, err := cmd.Output()
	if err != nil {
		log.Printf("🌡️ Temperature: ioreg failed: %v", err)
		return 0
	}
	re := regexp.MustCompile(`"current-value"\s*=\s*(\d+)`)
	matches := re.FindAllStringSubmatch(string(out), -1)
	var maxTemp int
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		raw, err := strconv.Atoi(m[1])
		if err != nil {
			continue
		}
		celsius := raw >> 16
		if celsius > 0 && celsius < 120 && celsius > maxTemp {
			maxTemp = celsius
		}
	}
	if maxTemp > 0 {
		return maxTemp * 1000
	}
	log.Printf("🌡️ Temperature: ioreg found no current-value (IOHWSensor)")
	return 0
}

func getTemperatureFromSystemProfiler() int {
	cmd := exec.Command("system_profiler", "SPSensorsDataType")
	out, err := cmd.Output()
	if err != nil {
		log.Printf("🌡️ Temperature: system_profiler failed: %v", err)
		return 0
	}
	re := regexp.MustCompile(`(\d{2,3})\s*°?\s*[Cc]`)
	matches := re.FindAllStringSubmatch(string(out), -1)
	var maxTemp int
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		t, err := strconv.Atoi(strings.TrimSpace(m[1]))
		if err != nil || t < 10 || t > 120 {
			continue
		}
		if t > maxTemp {
			maxTemp = t
		}
	}
	if maxTemp > 0 {
		return maxTemp * 1000
	}
	log.Printf("🌡️ Temperature: system_profiler found no valid temp")
	return 0
}

// getTemperatureFromPowermetrics tries powermetrics -s thermal (may require sudo).
func getTemperatureFromPowermetrics() int {
	cmd := exec.Command("powermetrics", "-s", "thermal", "-i", "1", "-n", "1")
	out, err := cmd.Output()
	if err != nil {
		log.Printf("🌡️ Temperature: powermetrics failed (may need sudo): %v", err)
		return 0
	}
	// Parse "CPU die temperature: 45 C" or similar
	re := regexp.MustCompile(`(?i)(?:die|package|cpu)\s*(?:die\s*)?temperature[:\s]+(\d+)`)
	matches := re.FindAllStringSubmatch(string(out), -1)
	var maxTemp int
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		t, err := strconv.Atoi(m[1])
		if err != nil || t < 10 || t > 120 {
			continue
		}
		if t > maxTemp {
			maxTemp = t
		}
	}
	if maxTemp > 0 {
		return maxTemp * 1000
	}
	// Fallback: any "XX C" or "XX°C" in thermal context
	re2 := regexp.MustCompile(`(\d{2,3})\s*°?\s*[Cc]`)
	for _, m := range re2.FindAllStringSubmatch(string(out), -1) {
		if len(m) < 2 {
			continue
		}
		t, _ := strconv.Atoi(strings.TrimSpace(m[1]))
		if t > 10 && t < 120 && t > maxTemp {
			maxTemp = t
		}
	}
	if maxTemp > 0 {
		return maxTemp * 1000
	}
	return 0
}
