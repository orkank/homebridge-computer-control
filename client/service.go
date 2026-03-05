package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"
)

// Auth token: getter/setter set from main (uses Fyne preferences).
// Client receives token from registration response; all incoming requests must include it.
var (
	getAuthToken     func() string
	setAuthToken     func(string)
	onUpdateAvailable func(string) // called when plugin reports update; opens download link on user action
	authTokenMu      sync.RWMutex
)

// RegistrationPayload is the JSON body sent to the Homebridge plugin.
type RegistrationPayload struct {
	Hostname   string `json:"hostname"`
	IP         string `json:"ip"`
	MAC        string `json:"mac"`
	Port       int    `json:"port"`
	OS         string `json:"os"`
	Arch       string `json:"arch,omitempty"`       // runtime.GOARCH for update platform selection
	Version   string `json:"version,omitempty"`    // client version for auto-update check
	IsDarkWake bool   `json:"isDarkWake,omitempty"` // macOS: true = Power Nap, plugin keeps device OFF
}

// SleepResponse is the JSON returned by the /sleep endpoint.
type SleepResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// StatusResponse is the JSON returned by the /status endpoint.
type StatusResponse struct {
	Status        string          `json:"status"`
	Hostname      string          `json:"hostname"`
	OS            string          `json:"os"`
	Uptime        string          `json:"uptime"`
	DisplayState  *DisplayStateInfo `json:"displayState,omitempty"`
}

// DisplayStateInfo holds display/power state for health/status (macOS).
type DisplayStateInfo struct {
	CurrentPowerState  int    `json:"currentPowerState"`  // 4=awake, 1=asleep
	IsDarkWake         bool   `json:"isDarkWake"`
	IsDisplayAsleep    bool   `json:"isDisplayAsleep"`
	DisplayAsleepCount int    `json:"displayAsleepCount"`
	DisplayAwakeCount  int    `json:"displayAwakeCount"`
	PowerStateSource   string `json:"powerStateSource"`
}

// ──────────────────────────────────────────────
// HTTP Server
// ──────────────────────────────────────────────

// requireAuth wraps a handler and returns 401 if X-Auth-Token is missing or invalid.
func requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("X-Auth-Token")
		authTokenMu.RLock()
		expected := ""
		if getAuthToken != nil {
			expected = getAuthToken()
		}
		authTokenMu.RUnlock()

		if expected == "" {
			// No token yet (client hasn't registered) — reject
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "No auth token configured"})
			return
		}
		if token != expected {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid auth token"})
			return
		}
		next(w, r)
	}
}

// startHTTPServer sets up and runs the HTTP server for sleep/status/health/wake-screen.
func startHTTPServer(hostname string) {
	mux := http.NewServeMux()
	mux.HandleFunc("/sleep", requireAuth(handleSleep))
	mux.HandleFunc("/status", requireAuth(handleStatus(hostname)))
	mux.HandleFunc("/health", requireAuth(handleHealth))
	mux.HandleFunc("/wake-screen", requireAuth(handleWakeScreen))
	mux.HandleFunc("/stay-awake", requireAuth(handleStayAwake))

	addr := fmt.Sprintf(":%d", flagPort)
	log.Printf("🚀 HTTP server listening on %s", addr)

	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Printf("❌ HTTP server failed: %v", err)
	}
}

// handleSleep executes the platform-specific sleep command.
func handleSleep(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	log.Println("💤 Sleep request received")
	w.Header().Set("Content-Type", "application/json")

	// Send "Going to Sleep" to Homebridge BEFORE sleeping so plugin can set device OFF immediately
	sendGoingToSleep(appState.MAC)

	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("osascript", "-e", `tell application "System Events" to sleep`)
	case "windows":
		cmd = exec.Command("rundll32.exe", "powrprof.dll,SetSuspendState", "0,1,0")
	case "linux":
		cmd = exec.Command("systemctl", "suspend")
	default:
		resp := SleepResponse{
			Success: false,
			Message: fmt.Sprintf("Unsupported OS: %s", runtime.GOOS),
		}
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(resp)
		return
	}

	// Send success BEFORE sleeping (machine won't respond after)
	resp := SleepResponse{
		Success: true,
		Message: fmt.Sprintf("Sleep initiated on %s", runtime.GOOS),
	}
	json.NewEncoder(w).Encode(resp)

	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}

	// Execute with slight delay so response is delivered
	go func() {
		time.Sleep(500 * time.Millisecond)
		if err := cmd.Run(); err != nil {
			log.Printf("⚠️  Sleep command error: %v", err)
		}
	}()
}

// sendGoingToSleep notifies Homebridge that this client is about to sleep.
// Plugin will immediately set the device to OFF.
func sendGoingToSleep(mac string) {
	if mac == "" || flagPluginURL == "" {
		return
	}
	payload := map[string]string{"mac": mac}
	body, err := json.Marshal(payload)
	if err != nil {
		return
	}
	url := strings.TrimRight(flagPluginURL, "/") + "/going-to-sleep"
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		log.Printf("⚠️  Going-to-sleep notify failed: %v", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		log.Println("📤 Going-to-sleep sent to Homebridge")
	}
}

// handleStatus returns the current client status with display state debug info.
func handleStatus(hostname string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		uptime := time.Since(startTime).Round(time.Second)
		resp := StatusResponse{
			Status:   "online",
			Hostname: hostname,
			OS:       runtime.GOOS,
			Uptime:   uptime.String(),
		}
		if runtime.GOOS == "darwin" {
			ds := getDisplayStateInfo()
			resp.DisplayState = &ds
		}
		json.NewEncoder(w).Encode(resp)
	}
}

// handleHealth returns liveness + display state. Plugin uses isDarkWake to decide ONLINE/OFFLINE.
func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	resp := map[string]interface{}{"status": "ok"}
	if runtime.GOOS == "darwin" {
		ds := getDisplayStateInfo()
		resp["displayState"] = ds
		resp["isDarkWake"] = ds.IsDarkWake
	}
	json.NewEncoder(w).Encode(resp)
}

// handleStayAwake enables or disables system sleep prevention on all clients.
func handleStayAwake(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	enabled := r.URL.Query().Get("enabled") == "true"
	w.Header().Set("Content-Type", "application/json")

	var ok bool
	if enabled {
		ok = startStayAwake()
	} else {
		ok = stopStayAwake()
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": ok,
		"message": map[bool]string{true: "Stay-awake enabled", false: "Stay-awake disabled"}[enabled],
	})
}

// handleWakeScreen runs caffeinate -u -t 2 to force display wake (macOS only).
// Called after WoL to ensure the screen turns on.
func handleWakeScreen(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	if runtime.GOOS != "darwin" {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"message": "wake-screen is macOS-only; no-op on " + runtime.GOOS,
		})
		return
	}

	log.Println("🖥️  Wake-screen request received (full wake: caffeinate + key + brightness)")
	go runFullWake()

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Display wake triggered",
	})
}

// runCaffeinateWake runs caffeinate to prevent display/system sleep.
// -d = display, -i = idle, -u = user active. 45s gives user time to interact after deep sleep wake.
func runCaffeinateWake() {
	cmd := exec.Command("caffeinate", "-d", "-i", "-u", "-t", "45")
	if err := cmd.Start(); err != nil {
		log.Printf("⚠️  caffeinate wake error: %v", err)
		return
	}
	go func() { _ = cmd.Wait() }()
}

// runKeyAndBrightness sends "user active" signal + max brightness.
func runKeyAndBrightness() {
	_ = exec.Command("osascript", "-e", `tell application "System Events" to key code 123`).Run()
	time.Sleep(100 * time.Millisecond)
	_ = exec.Command("brightness", "1").Run()
}

// runFullWake does caffeinate + repeated key/brightness for deep sleep wake.
// When Mac auto-sleeps (lid closed, idle), it enters deeper sleep. A single key press
// often fails; we run multiple attempts so the system has time to fully wake.
func runFullWake() {
	runCaffeinateWake() // 45s background, prevents display/system sleep
	time.Sleep(500 * time.Millisecond)
	runKeyAndBrightness()
	// Retry at 2s, 5s, 10s — deep sleep often needs multiple "user active" signals
	go func() {
		for _, d := range []time.Duration{2 * time.Second, 5 * time.Second, 10 * time.Second} {
			time.Sleep(d)
			runKeyAndBrightness()
		}
	}()
}

// runAutoDisplayWake runs full wake on startup (macOS only).
func runAutoDisplayWake() {
	if runtime.GOOS != "darwin" {
		return
	}
	go func() {
		time.Sleep(500 * time.Millisecond)
		log.Println("🖥️  Auto-display wake on startup")
		runFullWake()
	}()
}

// ──────────────────────────────────────────────
// Heartbeat / Registration
// ──────────────────────────────────────────────

// heartbeatLoop sends periodic registration to the Homebridge plugin.
func heartbeatLoop() {
	// Small delay to let the GUI render first
	time.Sleep(2 * time.Second)

	// Initial heartbeat (skip if sleeping or dark wake)
	if !shouldSkipHeartbeat() {
		ok := sendHeartbeat(appState.Hostname, appState.IP, appState.MAC)
		updateConnectionStatus(ok)
	}

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		// macOS: skip heartbeat if system is sleeping or display is in Dark Wake
		if shouldSkipHeartbeat() {
			updateConnectionStatus(false)
			continue
		}

		// Re-detect IP in case it changed (DHCP)
		newIP, newMAC, err := getNetworkInfo()
		if err != nil {
			log.Printf("⚠️  Network refresh failed: %v", err)
			updateConnectionStatus(false)
			continue
		}
		if newIP != appState.IP {
			log.Printf("🔄 IP changed: %s → %s", appState.IP, newIP)
			updateIPDisplay(newIP)
		}
		if newMAC != appState.MAC {
			log.Printf("🔄 MAC changed: %s → %s", appState.MAC, newMAC)
			appState.MAC = newMAC
		}

		ok := sendHeartbeat(appState.Hostname, appState.IP, appState.MAC)
		updateConnectionStatus(ok)
	}
}

// shouldSkipHeartbeat returns true if we should NOT send heartbeat.
// When screen is off (Dark Wake), client must stay silent - never send "I'm online".
func shouldSkipHeartbeat() bool {
	return isDisplayInDarkWake()
}

// sendHeartbeat sends a single registration request and returns success.
func sendHeartbeat(hostname, ip, mac string) bool {
	payload := RegistrationPayload{
		Hostname:   hostname,
		IP:         ip,
		MAC:        mac,
		Port:       flagPort,
		OS:         runtime.GOOS,
		Arch:       runtime.GOARCH,
		Version:    clientVersion,
		IsDarkWake: isDisplayInDarkWake(),
	}

	body, err := json.Marshal(payload)
	if err != nil {
		log.Printf("⚠️  Marshal error: %v", err)
		return false
	}

	url := strings.TrimRight(flagPluginURL, "/") + "/register"
	client := &http.Client{Timeout: 5 * time.Second}

	resp, err := client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		appendLog("⚠️ Heartbeat failed: " + err.Error())
		log.Printf("⚠️  Heartbeat failed: %v", err)
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		var result struct {
			Token           string `json:"token"`
			UpdateAvailable bool   `json:"updateAvailable"`
			UpdateURL       string `json:"updateUrl"`
			UpdateSha256    string `json:"updateSha256"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err == nil {
			if result.Token != "" {
				authTokenMu.Lock()
				if setAuthToken != nil {
					setAuthToken(result.Token)
				}
				authTokenMu.Unlock()
			}
			if result.UpdateAvailable && result.UpdateURL != "" {
				appendLog("📦 Update available: " + result.UpdateURL)
				if onUpdateAvailable != nil {
					onUpdateAvailable(result.UpdateURL)
				}
			}
		} else {
			appendLog("⚠️ Heartbeat: failed to parse JSON response")
		}
		log.Printf("💓 Heartbeat OK → %s", url)
		return true
	}

	appendLog("⚠️ Heartbeat failed: HTTP " + fmt.Sprintf("%d", resp.StatusCode))
	log.Printf("⚠️  Heartbeat returned %d", resp.StatusCode)
	return false
}
