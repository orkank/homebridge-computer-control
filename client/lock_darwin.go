//go:build darwin

package main

import (
	"log"
	"net/http"
	"os/exec"
)

// handleLockDarwin tries multiple methods to lock the screen on macOS.
// CGSession -suspend was removed in Big Sur. pmset displaysleepnow may not trigger lock.
// osascript with Cmd+Ctrl+Q requires Accessibility permission for the app.
func handleLockDarwin(w http.ResponseWriter, r *http.Request) {
	log.Printf("🔒 Lock: trying method 1 (osascript Cmd+Ctrl+Q)")
	appendLog("🔒 Lock: trying osascript key code...")

	// Method 1: osascript - simulates Cmd+Ctrl+Q (default Lock Screen shortcut).
	// Requires: System Settings > Privacy & Security > Accessibility > add Computer Control app.
	cmd := exec.Command("osascript", "-e", `tell application "System Events" to key code 12 using {control down, command down}`)
	err := cmd.Run()
	if err == nil {
		log.Printf("🔒 Lock: success (osascript key)")
		appendLog("🔒 Lock: success (osascript)")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"success":true,"message":"Screen locked"}`))
		return
	}
	log.Printf("⚠️  Lock method 1 failed: %v", err)
	appendLog("⚠️ Lock (osascript key) failed, trying Finder menu...")

	// Method 2: osascript - Finder Apple menu "Lock Screen" (shortcut-agnostic, works on Ventura+).
	cmd = exec.Command("osascript", "-e", `tell application "System Events" to tell process "Finder" to click menu item "Lock Screen" of menu 1 of menu bar 1`)
	if err := cmd.Run(); err == nil {
		log.Printf("🔒 Lock: success (Finder menu)")
		appendLog("🔒 Lock: success (Finder menu)")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"success":true,"message":"Screen locked"}`))
		return
	}
	log.Printf("⚠️  Lock method 2 failed")
	appendLog("⚠️ Lock (Finder menu) failed, trying pmset...")

	// Method 3: pmset displaysleepnow - turns off display. Locks when "Require password after sleep" is on.
	cmd = exec.Command("pmset", "displaysleepnow")
	if err := cmd.Run(); err == nil {
		log.Printf("🔒 Lock: success (pmset displaysleepnow)")
		appendLog("🔒 Lock: success (pmset)")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"success":true,"message":"Screen locked (display sleep)"}`))
		return
	}
	log.Printf("⚠️  Lock: all methods failed")
	appendLog("⚠️ Lock: all methods failed")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusInternalServerError)
	w.Write([]byte(`{"success":false,"error":"All lock methods failed. For osascript: grant Accessibility permission to the app in System Settings > Privacy & Security > Accessibility."}`))
}
