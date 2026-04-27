package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
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

// Anti-sleep timeout: if client cannot reach server for 10 min, turn off stay-awake locally.
const stayAwakeNoConnectionTimeout = 10 * time.Minute

var (
	lastSuccessfulHeartbeat time.Time
	stayAwakeActivatedAt    time.Time
	stayAwakeTimeoutMu      sync.Mutex
)

// RegistrationPayload is the JSON body sent to the Homebridge plugin.
type RegistrationPayload struct {
	Hostname             string            `json:"hostname"`
	IP                   string            `json:"ip"`
	MAC                  string            `json:"mac"`
	Port                 int               `json:"port"`
	OS                   string            `json:"os"`
	Arch                 string            `json:"arch,omitempty"`
	Version              string            `json:"version,omitempty"`
	IsDarkWake           bool              `json:"isDarkWake,omitempty"`
	Temperature          *int              `json:"temperature,omitempty"`
	Actions              []Action          `json:"actions,omitempty"`
	AppStates            map[string]bool    `json:"appStates,omitempty"`
		ManagedApps          []ManagedAppEntry `json:"managedApps,omitempty"`
		ScreensaverEnabled   bool              `json:"screensaverEnabled,omitempty"`
		LockEnabled          bool              `json:"lockEnabled,omitempty"`
		EnableVolumeSlider   bool              `json:"enableVolumeSlider,omitempty"`
		JoinMasterVolume     bool              `json:"joinMasterVolume,omitempty"`
		VolumeSliderName     string            `json:"volumeSliderName,omitempty"`
		Volume               *int              `json:"volume,omitempty"`
		EnableAntiSleep      bool              `json:"enableAntiSleep,omitempty"`
		JoinAntiSleep        bool              `json:"joinAntiSleep,omitempty"`
		EnableLockPrevention bool              `json:"enableLockPrevention,omitempty"`
		JoinLockPrevention   bool              `json:"joinLockPrevention,omitempty"`
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
	mux.HandleFunc("/run-action", requireAuth(handleRunAction))
	mux.HandleFunc("/manage-app", requireAuth(handleManageApp))
	mux.HandleFunc("/screensaver", requireAuth(handleScreensaver))
	mux.HandleFunc("/lock", requireAuth(handleLock))
	mux.HandleFunc("/volume", requireAuth(handleVolume))

	addr := fmt.Sprintf(":%d", flagPort)
	log.Printf("🚀 HTTP server listening on %s", addr)

	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Printf("❌ HTTP server failed: %v", err)
	}
}

// triggerSleep puts the computer to sleep. Used by handleSleep and by runAction when SleepAfterAction.
// macOS: pmset sleepnow. Windows: rundll32 powrprof. Linux: systemctl suspend.
func triggerSleep() {
	sendGoingToSleep(appState.MAC)
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("pmset", "sleepnow")
	case "windows":
		cmd = exec.Command("rundll32.exe", "powrprof.dll,SetSuspendState", "0,1,0")
		prepareCmd(cmd)
	case "linux":
		cmd = exec.Command("systemctl", "suspend")
	default:
		log.Printf("⚠️  Sleep not supported on %s", runtime.GOOS)
		return
	}
	time.Sleep(500 * time.Millisecond)
	if err := cmd.Run(); err != nil {
		if runtime.GOOS == "darwin" {
			// Fallback: osascript (works without pmset privileges)
			fallback := exec.Command("osascript", "-e", `tell application "System Events" to sleep`)
			if fallbackErr := fallback.Run(); fallbackErr != nil {
				log.Printf("⚠️  Sleep failed (pmset: %v, osascript: %v)", err, fallbackErr)
			}
		} else {
			log.Printf("⚠️  Sleep command error: %v", err)
		}
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
	if runtime.GOOS != "darwin" && runtime.GOOS != "windows" && runtime.GOOS != "linux" {
		json.NewEncoder(w).Encode(SleepResponse{Success: false, Message: fmt.Sprintf("Unsupported OS: %s", runtime.GOOS)})
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(SleepResponse{Success: true, Message: fmt.Sprintf("Sleep initiated on %s", runtime.GOOS)})
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
	go triggerSleep()
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

// recordStayAwakeActivated is called when stay-awake is turned on (for timeout grace period).
func recordStayAwakeActivated() {
	stayAwakeTimeoutMu.Lock()
	stayAwakeActivatedAt = time.Now()
	stayAwakeTimeoutMu.Unlock()
}

// recordHeartbeatResult updates lastSuccessfulHeartbeat and checks if stay-awake should auto-off.
// Call after each sendHeartbeat attempt (not when heartbeat is skipped).
func recordHeartbeatResult(ok bool) {
	stayAwakeTimeoutMu.Lock()
	if ok {
		lastSuccessfulHeartbeat = time.Now()
	}
	stayAwakeTimeoutMu.Unlock()

	// If anti-sleep is on but we haven't reached server for 10 min, turn off locally
	if !isStayAwakeActive() || flagPluginURL == "" {
		return
	}
	stayAwakeTimeoutMu.Lock()
	cutoff := lastSuccessfulHeartbeat
	if cutoff.IsZero() {
		cutoff = stayAwakeActivatedAt
	}
	elapsed := time.Since(cutoff)
	stayAwakeTimeoutMu.Unlock()

	if elapsed >= stayAwakeNoConnectionTimeout {
		log.Printf("☕ Anti-Sleep auto-off: no server connection for %.0f min", elapsed.Minutes())
		stopStayAwake()
	}
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
		if ok {
			recordStayAwakeActivated()
		}
	} else {
		ok = stopStayAwake()
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": ok,
		"message": map[bool]string{true: "Stay-awake enabled", false: "Stay-awake disabled"}[enabled],
	})
}

// handleRunAction executes a named action. GET /run-action?name=X&state=on|off
// Fire-and-forget: returns 200 OK immediately, runs in background.
func handleRunAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	name := r.URL.Query().Get("name")
	state := r.URL.Query().Get("state")
	if name == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Missing name parameter"})
		return
	}
	if state != "on" && state != "off" {
		state = "on"
	}

	actions := getActions()
	var found *Action
	for i := range actions {
		if actions[i].Name == name {
			found = &actions[i]
			break
		}
	}
	if found == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "Action not found"})
		return
	}

	// Fire and forget
	go runAction(*found, state)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Action started",
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
		recordHeartbeatResult(ok)
		updateConnectionStatus(ok)
	}

	for {
		interval := time.Duration(getHeartbeatIntervalSec()) * time.Second
		ticker := time.NewTicker(interval)
		<-ticker.C
		ticker.Stop()

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
		recordHeartbeatResult(ok)
		updateConnectionStatus(ok)
	}
}

// shouldSkipHeartbeat returns true if we should NOT send heartbeat.
// When screen is off (Dark Wake), client must stay silent - never send "I'm online".
func shouldSkipHeartbeat() bool {
	return isDisplayInDarkWake()
}

var lastImmediateHeartbeat time.Time
var immediateHeartbeatMu sync.Mutex

// TriggerImmediateHeartbeat sends a heartbeat right away when settings change.
// Debounced: skips if last trigger was < 2s ago (e.g. rapid volume name typing).
func TriggerImmediateHeartbeat() {
	immediateHeartbeatMu.Lock()
	if time.Since(lastImmediateHeartbeat) < 2*time.Second {
		immediateHeartbeatMu.Unlock()
		return
	}
	lastImmediateHeartbeat = time.Now()
	immediateHeartbeatMu.Unlock()

	if flagPluginURL == "" || shouldSkipHeartbeat() {
		return
	}
	go func() {
		ok := sendHeartbeat(appState.Hostname, appState.IP, appState.MAC)
		recordHeartbeatResult(ok)
		updateConnectionStatus(ok)
	}()
}

// volumeSyncLoop polls volume when volume slider is enabled; notifies plugin on change.
func volumeSyncLoop() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		if (!getJoinMasterVolume() && !getEnableVolumeSlider()) || flagPluginURL == "" {
			continue
		}
		if shouldSkipHeartbeat() {
			continue
		}

		v := GetVolumeLevel()
		if v < 0 {
			continue
		}

		lastSentVolumeMu.RLock()
		last := lastSentVolume
		lastSentVolumeMu.RUnlock()

		if v == last {
			continue
		}

		// Volume changed locally — notify plugin (master: broadcast; individual: update slider)
		if sendVolumeChangedToPlugin(appState.MAC, v) {
			lastSentVolumeMu.Lock()
			lastSentVolume = v
			lastSentVolumeMu.Unlock()
		}
	}
}

// sendVolumeChangedToPlugin notifies the plugin that this device's volume changed.
// Plugin will broadcast the new level to all other devices in the master volume group.
func sendVolumeChangedToPlugin(mac string, level int) bool {
	if mac == "" || flagPluginURL == "" {
		return false
	}

	authTokenMu.RLock()
	token := ""
	if getAuthToken != nil {
		token = getAuthToken()
	}
	authTokenMu.RUnlock()

	payload := map[string]interface{}{"mac": mac, "level": level}
	body, err := json.Marshal(payload)
	if err != nil {
		return false
	}

	url := strings.TrimRight(flagPluginURL, "/") + "/volume-changed"
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return false
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("X-Auth-Token", token)
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("⚠️  Volume-changed notify failed: %v", err)
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		log.Printf("🔊 Volume changed → %d%% (broadcast to master group)", level)
		return true
	}
	return false
}

// sendHeartbeat sends a single registration request and returns success.
func sendHeartbeat(hostname, ip, mac string) bool {
	// Compute app_states from managed apps and process list
	managedApps := getManagedApps()
	appStates := make(map[string]bool)
	for _, app := range managedApps {
		appStates[app.Name] = IsProcessRunning(app.Name)
	}

	payload := RegistrationPayload{
		Hostname:           hostname,
		IP:                 ip,
		MAC:                mac,
		Port:               flagPort,
		OS:                 runtime.GOOS,
		Arch:               runtime.GOARCH,
		Version:            clientVersion,
		IsDarkWake:         isDisplayInDarkWake(),
		Actions:            getActions(),
		AppStates:          appStates,
		ManagedApps:        managedApps,
		ScreensaverEnabled:   getEnableRemoteScreensaver(),
		LockEnabled:          getEnableRemoteLock(),
		EnableVolumeSlider:   getEnableVolumeSlider(),
		JoinMasterVolume:     getJoinMasterVolume(),
		VolumeSliderName:     getVolumeSliderName(),
		EnableAntiSleep:      getEnableAntiSleep(),
		JoinAntiSleep:        getJoinAntiSleep(),
		EnableLockPrevention: getEnableLockPrevention(),
		JoinLockPrevention:   getJoinLockPrevention(),
	}
	// Volume in heartbeat only for individual slider; master volume uses volume-changed when it changes
	if getEnableVolumeSlider() {
		if v := GetVolumeLevel(); v >= 0 {
			payload.Volume = &v
		}
	}
	// Only read and send temperature when user enabled "Send Temperature Data" (minimizes CPU load)
	// On failure: send null silently, no log spam
	if getSendTemperature() {
		if t := getCPUTemperatureMillidegree(); t > 0 {
			payload.Temperature = &t
		}
	}

	body, err := json.Marshal(payload)
	if err != nil {
		log.Printf("⚠️  Marshal error: %v", err)
		return false
	}

	log.Printf("💓 Heartbeat request: %s", string(body))

	if onHeartbeatSending != nil {
		onHeartbeatSending(true)
		defer func() {
			if onHeartbeatSending != nil {
				onHeartbeatSending(false)
			}
		}()
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
		respBody, _ := io.ReadAll(resp.Body)
		log.Printf("💓 Heartbeat response: %s", string(respBody))

		var result struct {
			Token           string `json:"token"`
			UpdateAvailable bool   `json:"updateAvailable"`
			UpdateURL       string `json:"updateUrl"`
			UpdateSha256    string `json:"updateSha256"`
		}
		if err := json.NewDecoder(bytes.NewReader(respBody)).Decode(&result); err == nil {
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
