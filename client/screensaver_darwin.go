//go:build darwin

package main

import (
	"log"
	"net/http"
	"os/exec"
)

func init() {
	handleScreensaverDarwinFunc = handleScreensaverDarwin
}

// handleScreensaverDarwin tries multiple methods to start the screensaver on macOS.
// open -a ScreenSaverEngine worked before recent changes; try it first.
func handleScreensaverDarwin(w http.ResponseWriter, r *http.Request) {
	// Method 1: App name - classic method (was working before).
	log.Printf("🖼️ Screensaver: trying method 1 (open -a ScreenSaverEngine)")
	appendLog("🖼️ Screensaver: trying open -a ScreenSaverEngine...")
	cmd := exec.Command("open", "-a", "ScreenSaverEngine")
	err := cmd.Start()
	if err == nil {
		go func() { _ = cmd.Wait() }()
		log.Printf("🖼️ Screensaver: success (open -a)")
		appendLog("🖼️ Screensaver: success")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"success":true,"message":"Screensaver started"}`))
		return
	}
	log.Printf("⚠️  Screensaver method 1 failed: %v", err)
	appendLog("⚠️ Screensaver (open -a) failed, trying full path...")

	// Method 2: Full path - for Sonoma+.
	cmd = exec.Command("open", "/System/Library/CoreServices/ScreenSaverEngine.app")
	err = cmd.Start()
	if err == nil {
		go func() { _ = cmd.Wait() }()
		log.Printf("🖼️ Screensaver: success (full path)")
		appendLog("🖼️ Screensaver: success (full path)")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"success":true,"message":"Screensaver started"}`))
		return
	}
	log.Printf("⚠️  Screensaver method 2 failed: %v", err)
	appendLog("⚠️ Screensaver (full path) failed, trying bundle ID...")

	// Method 3: Bundle ID - works on Monterey+.
	cmd = exec.Command("open", "-b", "com.apple.ScreenSaver.Engine")
	err = cmd.Start()
	if err == nil {
		go func() { _ = cmd.Wait() }()
		log.Printf("🖼️ Screensaver: success (bundle ID)")
		appendLog("🖼️ Screensaver: success (bundle ID)")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"success":true,"message":"Screensaver started"}`))
		return
	}
	log.Printf("⚠️  Screensaver method 3 failed: %v", err)
	appendLog("⚠️ Screensaver (bundle ID) failed, trying direct exec...")

	// Method 4: Direct executable - last resort.
	cmd = exec.Command("/System/Library/CoreServices/ScreenSaverEngine.app/Contents/MacOS/ScreenSaverEngine")
	err = cmd.Start()
	if err == nil {
		go func() { _ = cmd.Wait() }()
		log.Printf("🖼️ Screensaver: success (direct exec)")
		appendLog("🖼️ Screensaver: success (direct exec)")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"success":true,"message":"Screensaver started"}`))
		return
	}
	log.Printf("⚠️  Screensaver: all methods failed: %v", err)
	appendLog("⚠️ Screensaver: all methods failed")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusInternalServerError)
	w.Write([]byte(`{"success":false,"error":"All screensaver methods failed"}`))
}
