//go:build windows

package main

import (
	"os/exec"
	"strconv"
	"strings"
)

// getCPUTemperatureMillidegree returns CPU temperature in millidegrees Celsius, or 0 if unavailable.
// Tries multiple WMI classes; no logging on failure (silent).
func getCPUTemperatureMillidegree() int {
	// 1. MSAcpi_ThermalZoneTemperature (ROOT\WMI) - tenths of Kelvin
	if v := queryWmiTemp(`Get-CimInstance -Namespace root/wmi -ClassName MSAcpi_ThermalZoneTemperature -ErrorAction SilentlyContinue | Select-Object -First 1 -ExpandProperty CurrentTemperature`); v > 0 {
		return v
	}
	// 2. Win32_PerfFormattedData_Counters_ThermalZoneInformation (ROOT\CIMV2)
	if v := queryWmiTemp(`Get-CimInstance -ClassName Win32_PerfFormattedData_Counters_ThermalZoneInformation -Namespace root/cimv2 -ErrorAction SilentlyContinue | Select-Object -First 1 -ExpandProperty HighPrecisionTemperature`); v > 0 {
		return v
	}
	// 3. Win32_TemperatureProbe (if available)
	if v := queryWmiTemp(`Get-CimInstance -ClassName Win32_TemperatureProbe -ErrorAction SilentlyContinue | Select-Object -First 1 -ExpandProperty CurrentReading`); v > 0 {
		return v
	}
	return 0
}

// queryWmiTemp runs PowerShell and converts tenths-of-Kelvin to millidegree Celsius.
// (value/10) - 273.15 = Celsius
func queryWmiTemp(ps string) int {
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-Command", ps)
	prepareCmd(cmd)
	out, err := cmd.Output()
	if err != nil {
		return 0
	}
	s := strings.TrimSpace(string(out))
	if s == "" {
		return 0
	}
	val, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}
	// Tenths of Kelvin -> Celsius = (val/10) - 273.15
	kelvinTenths := float64(val)
	celsius := (kelvinTenths / 10.0) - 273.15
	if celsius < 0 || celsius > 120 {
		return 0
	}
	return int(celsius * 1000)
}
