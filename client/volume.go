package main

import (
	"net/http"
	"strconv"
	"strings"
	"sync"
)

var (
	lastSentVolume   int = -1
	lastSentVolumeMu sync.RWMutex
)

// updateLastSentVolumeWhenSet is called when volume is set via /volume (e.g. from plugin).
// Prevents redundant volume-changed notifications for plugin-initiated changes.
func updateLastSentVolumeWhenSet(level int) {
	lastSentVolumeMu.Lock()
	lastSentVolume = level
	lastSentVolumeMu.Unlock()
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// handleVolume handles GET /volume?level=[0-100] (set) or GET /volume (get current).
// Setting requires Enable Volume Slider.
func handleVolume(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")

	levelStr := strings.TrimSpace(r.URL.Query().Get("level"))
	if levelStr == "" {
		// GET current volume (for status sync)
		lvl := GetVolumeLevel()
		if lvl < 0 {
			w.Write([]byte(`{"success":true,"level":null}`))
			return
		}
		w.Write([]byte(`{"success":true,"level":` + strconv.Itoa(lvl) + `}`))
		return
	}

	if !getEnableVolumeSlider() && !getJoinMasterVolume() {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"success":false,"error":"Volume slider is disabled in client settings"}`))
		return
	}

	level, err := strconv.Atoi(levelStr)
	if err != nil || level < 0 || level > 100 {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"success":false,"error":"Level must be 0-100"}`))
		return
	}

	if err := setVolumeLevel(level); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"success":false,"error":"` + err.Error() + `"}`))
		return
	}

	updateLastSentVolumeWhenSet(level)
	w.Write([]byte(`{"success":true,"message":"Volume set","level":` + strconv.Itoa(level) + `}`))
}

var getVolumeLevelImpl = func() int { return -1 }

// GetVolumeLevel returns current volume 0-100, or -1 if unsupported/failed.
func GetVolumeLevel() int {
	return getVolumeLevelImpl()
}
