package main

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

// ClientConfig holds persisted client preferences.
type ClientConfig struct {
	SendTemperature          bool   `json:"sendTemperature"`
	EnableRemoteScreensaver   bool   `json:"enableRemoteScreensaver"`
	EnableRemoteLock          bool   `json:"enableRemoteLock"`
	EnableVolumeSlider        bool   `json:"enableVolumeSlider"`
	JoinMasterVolume          bool   `json:"joinMasterVolume"`
	VolumeSliderName          string `json:"volumeSliderName"` // default: [Hostname] - Volume
	EnableAntiSleep           bool   `json:"enableAntiSleep"`           // individual anti-sleep switch
	JoinAntiSleep             bool   `json:"joinAntiSleep"`             // join global anti-sleep (mutual excl with EnableAntiSleep)
	EnableLockPrevention      bool   `json:"enableLockPrevention"`      // individual lock prevention
	JoinLockPrevention        bool   `json:"joinLockPrevention"`        // join global lock prevention (mutual excl with EnableLockPrevention)
	HeartbeatIntervalSec      int    `json:"heartbeatIntervalSec"`      // default: 30
}

var (
	config     ClientConfig
	configPath string
	configMu   sync.RWMutex
)

func init() {
	configPath = getClientConfigPath()
}

// GetConfigDir returns the config directory (shared by actions, managed_apps, etc.).
func GetConfigDir() string {
	return filepath.Dir(getClientConfigPath())
}

// getClientConfigPath returns the path to client_config.json.
// Uses OS-appropriate config directory.
func getClientConfigPath() string {
	var dir string
	switch runtime.GOOS {
	case "darwin":
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, "Library", "Application Support", "Computer Control")
	case "windows":
		dir = os.Getenv("APPDATA")
		if dir == "" {
			home, _ := os.UserHomeDir()
			dir = filepath.Join(home, "AppData", "Roaming")
		}
		dir = filepath.Join(dir, "Computer Control")
	default:
		configDir := os.Getenv("XDG_CONFIG_HOME")
		if configDir == "" {
			home, _ := os.UserHomeDir()
			configDir = filepath.Join(home, ".config")
		}
		dir = filepath.Join(configDir, "computer-control")
	}
	return filepath.Join(dir, "client_config.json")
}

// loadClientConfig loads client_config.json. Creates default config if missing.
func loadClientConfig() ClientConfig {
	data, err := os.ReadFile(configPath)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("⚠️  Failed to read client_config: %v", err)
		}
		return ClientConfig{SendTemperature: false, EnableRemoteScreensaver: false, EnableRemoteLock: false, EnableVolumeSlider: false, JoinMasterVolume: false, EnableAntiSleep: false, JoinAntiSleep: false, EnableLockPrevention: false, JoinLockPrevention: false, HeartbeatIntervalSec: 30}
	}

	var c ClientConfig
	if err := json.Unmarshal(data, &c); err != nil {
		log.Printf("⚠️  Failed to parse client_config: %v", err)
		return ClientConfig{SendTemperature: false, EnableRemoteScreensaver: false, EnableRemoteLock: false, EnableVolumeSlider: false, JoinMasterVolume: false, EnableAntiSleep: false, JoinAntiSleep: false, EnableLockPrevention: false, JoinLockPrevention: false, HeartbeatIntervalSec: 30}
	}
	if c.HeartbeatIntervalSec <= 0 {
		c.HeartbeatIntervalSec = 30
	}
	if c.HeartbeatIntervalSec < 5 {
		c.HeartbeatIntervalSec = 5
	}
	if c.HeartbeatIntervalSec > 300 {
		c.HeartbeatIntervalSec = 300
	}
	return c
}

// saveClientConfig persists the config to client_config.json.
func saveClientConfig(c ClientConfig) {
	configMu.Lock()
	config = c
	configMu.Unlock()

	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Printf("⚠️  Failed to create config dir: %v", err)
		return
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		log.Printf("⚠️  Failed to marshal client_config: %v", err)
		return
	}
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		log.Printf("⚠️  Failed to write client_config: %v", err)
	}
}

// getSendTemperature returns whether to send temperature data (thread-safe).
func getSendTemperature() bool {
	configMu.RLock()
	defer configMu.RUnlock()
	return config.SendTemperature
}

// setSendTemperature updates the preference and persists it.
func setSendTemperature(enabled bool) {
	configMu.Lock()
	config.SendTemperature = enabled
	c := config
	configMu.Unlock()
	saveClientConfig(c)
	TriggerImmediateHeartbeat()
}

// getEnableRemoteScreensaver returns whether remote screensaver is enabled.
func getEnableRemoteScreensaver() bool {
	configMu.RLock()
	defer configMu.RUnlock()
	return config.EnableRemoteScreensaver
}

// setEnableRemoteScreensaver updates the preference and persists it.
func setEnableRemoteScreensaver(enabled bool) {
	configMu.Lock()
	config.EnableRemoteScreensaver = enabled
	c := config
	configMu.Unlock()
	saveClientConfig(c)
	TriggerImmediateHeartbeat()
}

// getEnableRemoteLock returns whether remote lock is enabled.
func getEnableRemoteLock() bool {
	configMu.RLock()
	defer configMu.RUnlock()
	return config.EnableRemoteLock
}

// setEnableRemoteLock updates the preference and persists it.
func setEnableRemoteLock(enabled bool) {
	configMu.Lock()
	config.EnableRemoteLock = enabled
	c := config
	configMu.Unlock()
	saveClientConfig(c)
	TriggerImmediateHeartbeat()
}

// getEnableVolumeSlider returns whether volume slider is enabled.
func getEnableVolumeSlider() bool {
	configMu.RLock()
	defer configMu.RUnlock()
	return config.EnableVolumeSlider
}

// setEnableVolumeSlider updates the preference and persists it.
// When enabled, Join Master Volume is automatically disabled (mutual exclusion).
func setEnableVolumeSlider(enabled bool) {
	configMu.Lock()
	config.EnableVolumeSlider = enabled
	if enabled {
		config.JoinMasterVolume = false
	}
	c := config
	configMu.Unlock()
	saveClientConfig(c)
	TriggerImmediateHeartbeat()
}

// getJoinMasterVolume returns whether to join master volume group.
func getJoinMasterVolume() bool {
	configMu.RLock()
	defer configMu.RUnlock()
	return config.JoinMasterVolume
}

// setJoinMasterVolume updates the preference and persists it.
// When enabled, Enable Volume Slider is automatically disabled (mutual exclusion).
func setJoinMasterVolume(enabled bool) {
	configMu.Lock()
	config.JoinMasterVolume = enabled
	if enabled {
		config.EnableVolumeSlider = false
	}
	c := config
	configMu.Unlock()
	saveClientConfig(c)
	TriggerImmediateHeartbeat()
}

// getVolumeSliderName returns the display name for this device's volume slider.
func getVolumeSliderName() string {
	configMu.RLock()
	defer configMu.RUnlock()
	return config.VolumeSliderName
}

// setVolumeSliderName updates the preference and persists it.
func setVolumeSliderName(name string) {
	configMu.Lock()
	config.VolumeSliderName = name
	c := config
	configMu.Unlock()
	saveClientConfig(c)
	TriggerImmediateHeartbeat()
}

// getEnableAntiSleep returns whether individual anti-sleep is enabled.
func getEnableAntiSleep() bool {
	configMu.RLock()
	defer configMu.RUnlock()
	return config.EnableAntiSleep
}

// setEnableAntiSleep updates the preference. When enabled, JoinAntiSleep is disabled (mutual exclusion).
func setEnableAntiSleep(enabled bool) {
	configMu.Lock()
	config.EnableAntiSleep = enabled
	if enabled {
		config.JoinAntiSleep = false
	}
	c := config
	configMu.Unlock()
	saveClientConfig(c)
	TriggerImmediateHeartbeat()
}

// getJoinAntiSleep returns whether to join global anti-sleep.
func getJoinAntiSleep() bool {
	configMu.RLock()
	defer configMu.RUnlock()
	return config.JoinAntiSleep
}

// setJoinAntiSleep updates the preference. When enabled, EnableAntiSleep is disabled (mutual exclusion).
func setJoinAntiSleep(enabled bool) {
	configMu.Lock()
	config.JoinAntiSleep = enabled
	if enabled {
		config.EnableAntiSleep = false
	}
	c := config
	configMu.Unlock()
	saveClientConfig(c)
	TriggerImmediateHeartbeat()
}

// getEnableLockPrevention returns whether individual lock prevention is enabled.
func getEnableLockPrevention() bool {
	configMu.RLock()
	defer configMu.RUnlock()
	return config.EnableLockPrevention
}

// setEnableLockPrevention updates the preference. When enabled, JoinLockPrevention is disabled (mutual exclusion).
func setEnableLockPrevention(enabled bool) {
	configMu.Lock()
	config.EnableLockPrevention = enabled
	if enabled {
		config.JoinLockPrevention = false
	}
	c := config
	configMu.Unlock()
	saveClientConfig(c)
	TriggerImmediateHeartbeat()
}

// getJoinLockPrevention returns whether to join global lock prevention.
func getJoinLockPrevention() bool {
	configMu.RLock()
	defer configMu.RUnlock()
	return config.JoinLockPrevention
}

// setJoinLockPrevention updates the preference. When enabled, EnableLockPrevention is disabled (mutual exclusion).
func setJoinLockPrevention(enabled bool) {
	configMu.Lock()
	config.JoinLockPrevention = enabled
	if enabled {
		config.EnableLockPrevention = false
	}
	c := config
	configMu.Unlock()
	saveClientConfig(c)
	TriggerImmediateHeartbeat()
}

// getHeartbeatIntervalSec returns heartbeat interval in seconds (5-300).
func getHeartbeatIntervalSec() int {
	configMu.RLock()
	defer configMu.RUnlock()
	v := config.HeartbeatIntervalSec
	if v < 5 {
		return 5
	}
	if v > 300 {
		return 300
	}
	return v
}

// setHeartbeatIntervalSec updates the preference and persists it.
func setHeartbeatIntervalSec(sec int) {
	if sec < 5 {
		sec = 5
	}
	if sec > 300 {
		sec = 300
	}
	configMu.Lock()
	config.HeartbeatIntervalSec = sec
	c := config
	configMu.Unlock()
	saveClientConfig(c)
	TriggerImmediateHeartbeat()
}

// initClientConfig loads config at startup. Call from main.
func initClientConfig() {
	config = loadClientConfig()
}
