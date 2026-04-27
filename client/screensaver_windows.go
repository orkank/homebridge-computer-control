//go:build windows

package main

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows/registry"
)

// getWindowsScreensaverPath returns the path to the user's configured screensaver.
// Reads from HKCU\Control Panel\Desktop\SCRNSAVE.EXE.
// Falls back to scrnsave.scr if none configured or path invalid.
func getWindowsScreensaverPath() string {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Control Panel\Desktop`, registry.QUERY_VALUE)
	if err != nil {
		log.Printf("⚠️  Screensaver: could not open registry: %v", err)
		return defaultScreensaverPath()
	}
	defer k.Close()

	val, _, err := k.GetStringValue("SCRNSAVE.EXE")
	if err != nil || strings.TrimSpace(val) == "" {
		return defaultScreensaverPath()
	}

	val = strings.TrimSpace(val)
	// Value can be full path (C:\Windows\System32\bubbles.scr) or just filename (bubbles.scr)
	if filepath.IsAbs(val) {
		if _, err := os.Stat(val); err == nil {
			return val
		}
		log.Printf("⚠️  Screensaver: configured path not found: %s", val)
		return defaultScreensaverPath()
	}

	// Filename only - try System32
	system32 := filepath.Join(os.Getenv("SystemRoot"), "System32", val)
	if _, err := os.Stat(system32); err == nil {
		return system32
	}
	// Try SysWOW64 for 32-bit screensavers on 64-bit Windows
	syswow64 := filepath.Join(os.Getenv("SystemRoot"), "SysWOW64", val)
	if _, err := os.Stat(syswow64); err == nil {
		return syswow64
	}
	log.Printf("⚠️  Screensaver: %s not found in System32/SysWOW64", val)
	return defaultScreensaverPath()
}

func defaultScreensaverPath() string {
	return filepath.Join(os.Getenv("SystemRoot"), "System32", "scrnsave.scr")
}

// startWindowsScreensaver runs the user-configured screensaver.
func startWindowsScreensaver() *exec.Cmd {
	path := getWindowsScreensaverPath()
	cmd := exec.Command(path, "/s")
	prepareCmd(cmd)
	return cmd
}
