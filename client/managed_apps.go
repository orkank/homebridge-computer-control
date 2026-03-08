package main

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// ManagedAppEntry holds a single managed app with wake/sleep options.
type ManagedAppEntry struct {
	Name       string `json:"name"`
	WakeBefore bool   `json:"wakeBefore,omitempty"`
	SleepAfter bool   `json:"sleepAfter,omitempty"`
}

// ManagedAppsConfig holds the persisted managed apps list.
type ManagedAppsConfig struct {
	Apps []ManagedAppEntry `json:"apps"`
}

var (
	managedAppsConfig ManagedAppsConfig
	managedAppsPath   string
	managedAppsMu     sync.RWMutex
)

func init() {
	managedAppsPath = getManagedAppsConfigPath()
}

func getManagedAppsConfigPath() string {
	return filepath.Join(GetConfigDir(), "managed_apps.json")
}

// managedAppsConfigJSON supports migration from old []string format.
type managedAppsConfigJSON struct {
	Apps interface{} `json:"apps"` // []string or []ManagedAppEntry
}

func loadManagedAppsConfig() ManagedAppsConfig {
	data, err := os.ReadFile(managedAppsPath)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("⚠️  Failed to read managed_apps.json: %v", err)
		}
		return ManagedAppsConfig{Apps: []ManagedAppEntry{}}
	}
	var raw managedAppsConfigJSON
	if err := json.Unmarshal(data, &raw); err != nil {
		log.Printf("⚠️  Failed to parse managed_apps.json: %v", err)
		return ManagedAppsConfig{Apps: []ManagedAppEntry{}}
	}
	if raw.Apps == nil {
		return ManagedAppsConfig{Apps: []ManagedAppEntry{}}
	}
	// Migration: old format was []string
	if names, ok := raw.Apps.([]interface{}); ok && len(names) > 0 {
		if _, ok := names[0].(string); ok {
			var entries []ManagedAppEntry
			for _, n := range names {
				if s, ok := n.(string); ok && s != "" {
					entries = append(entries, ManagedAppEntry{Name: s})
				}
			}
			return ManagedAppsConfig{Apps: entries}
		}
	}
	var c ManagedAppsConfig
	if err := json.Unmarshal(data, &c); err != nil {
		return ManagedAppsConfig{Apps: []ManagedAppEntry{}}
	}
	if c.Apps == nil {
		c.Apps = []ManagedAppEntry{}
	}
	return c
}

func saveManagedAppsConfig(c ManagedAppsConfig) {
	managedAppsMu.Lock()
	managedAppsConfig = c
	managedAppsMu.Unlock()

	dir := filepath.Dir(managedAppsPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Printf("⚠️  Failed to create managed apps dir: %v", err)
		return
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		log.Printf("⚠️  Failed to marshal managed_apps.json: %v", err)
		return
	}
	if err := os.WriteFile(managedAppsPath, data, 0644); err != nil {
		log.Printf("⚠️  Failed to write managed_apps.json: %v", err)
	}
}

func getManagedApps() []ManagedAppEntry {
	managedAppsMu.RLock()
	defer managedAppsMu.RUnlock()
	out := make([]ManagedAppEntry, len(managedAppsConfig.Apps))
	copy(out, managedAppsConfig.Apps)
	return out
}

func setManagedApps(apps []ManagedAppEntry) {
	saveManagedAppsConfig(ManagedAppsConfig{Apps: apps})
}

func getManagedAppByName(name string) *ManagedAppEntry {
	managedAppsMu.RLock()
	defer managedAppsMu.RUnlock()
	n := strings.TrimSpace(name)
	for i := range managedAppsConfig.Apps {
		if strings.EqualFold(managedAppsConfig.Apps[i].Name, n) {
			e := managedAppsConfig.Apps[i]
			return &e
		}
	}
	return nil
}

func initManagedAppsConfig() {
	managedAppsConfig = loadManagedAppsConfig()
}
