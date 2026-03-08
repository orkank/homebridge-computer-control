package main

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

// ActionType defines the type of automation.
const (
	ActionTypeBTTTrigger = "btt_trigger"
	ActionTypeShell      = "shell"
	ActionTypeBatch     = "batch"
	ActionTypeAppleScript = "applescript"
	ActionTypeURL       = "url"
)

// ActionInterface defines how the action appears in HomeKit.
const (
	ActionInterfaceToggle = "toggle"
	ActionInterfaceButton = "button"
)

// URLMode for URL action type.
const (
	URLModeFetch   = "fetch"   // HTTP GET in background (async)
	URLModeBrowser = "browser" // Open in default browser
)

// Action represents a single automation action.
type Action struct {
	Name              string `json:"name"`
	Type              string `json:"type"`      // btt_trigger, shell, batch, applescript, url
	Value             string `json:"value"`     // command, script path, or URL
	Interface         string `json:"interface"` // toggle or button
	URLMode           string `json:"urlMode,omitempty"`
	WakeBeforeAction bool `json:"wakeBefore,omitempty"`
	SleepAfterAction bool `json:"sleepAfter,omitempty"`
}

// ActionTypes for dropdown (all platforms)
var ActionTypes = []string{ActionTypeBTTTrigger, ActionTypeShell, ActionTypeBatch, ActionTypeAppleScript, ActionTypeURL}

// ActionTypesForPlatform returns action types available on the current OS.
// macOS: all. Linux: shell, batch, url. Windows: batch, url only (no shell — use batch for .bat/.cmd).
func ActionTypesForPlatform() []string {
	switch runtime.GOOS {
	case "darwin":
		return ActionTypes
	case "windows":
		return []string{ActionTypeBatch, ActionTypeURL}
	default:
		return []string{ActionTypeShell, ActionTypeBatch, ActionTypeURL}
	}
}

// ActionInterfaces for dropdown (display labels)
var ActionInterfaces = []string{ActionInterfaceToggle, ActionInterfaceButton}

// ActionInterfaceLabels for UI display
var ActionInterfaceLabels = map[string]string{
	ActionInterfaceToggle: "On/Off Switch (remembers state)",
	ActionInterfaceButton:  "Tap Button (runs on tap, no state)",
}

// URLModeLabels for UI display when type is url
var URLModeLabels = map[string]string{
	URLModeFetch:   "HTTP request (async)",
	URLModeBrowser: "Open in default browser",
}

// NormalizeActionType returns a valid type string.
func NormalizeActionType(t string) string {
	t = strings.TrimSpace(strings.ToLower(t))
	for _, v := range ActionTypes {
		if t == v {
			return v
		}
	}
	return ActionTypeShell
}

// NormalizeActionInterface returns a valid interface string.
func NormalizeActionInterface(i string) string {
	i = strings.TrimSpace(strings.ToLower(i))
	for _, v := range ActionInterfaces {
		if i == v {
			return v
		}
	}
	return ActionInterfaceToggle
}

// ActionsConfig holds the persisted actions list.
type ActionsConfig struct {
	Actions []Action `json:"actions"`
}

var (
	actionsConfig     ActionsConfig
	actionsPath       string
	actionsMu         sync.RWMutex
)

func init() {
	actionsPath = getActionsConfigPath()
}

func getActionsConfigPath() string {
	switch runtime.GOOS {
	case "darwin":
		home, _ := os.UserHomeDir()
		return filepath.Join(home, "Library", "Application Support", "Computer Control", "actions.json")
	case "windows":
		dir := os.Getenv("APPDATA")
		if dir == "" {
			home, _ := os.UserHomeDir()
			dir = filepath.Join(home, "AppData", "Roaming")
		}
		return filepath.Join(dir, "Computer Control", "actions.json")
	default:
		configDir := os.Getenv("XDG_CONFIG_HOME")
		if configDir == "" {
			home, _ := os.UserHomeDir()
			configDir = filepath.Join(home, ".config")
		}
		return filepath.Join(configDir, "computer-control", "actions.json")
	}
}

// actionJSON supports both wakeBefore/wakeBeforeAction and sleepAfter/sleepAfterAction for backward compat.
type actionJSON struct {
	Name              string `json:"name"`
	Type              string `json:"type"`
	Value             string `json:"value"`
	Interface         string `json:"interface"`
	URLMode           string `json:"urlMode,omitempty"`
	WakeBefore        bool   `json:"wakeBefore,omitempty"`
	WakeBeforeAction  bool   `json:"wakeBeforeAction,omitempty"`
	SleepAfter        bool   `json:"sleepAfter,omitempty"`
	SleepAfterAction  bool   `json:"sleepAfterAction,omitempty"`
}

func (a *Action) UnmarshalJSON(data []byte) error {
	var j actionJSON
	if err := json.Unmarshal(data, &j); err != nil {
		return err
	}
	a.Name = j.Name
	a.Type = j.Type
	a.Value = j.Value
	a.Interface = j.Interface
	a.URLMode = j.URLMode
	a.WakeBeforeAction = j.WakeBefore || j.WakeBeforeAction
	a.SleepAfterAction = j.SleepAfter || j.SleepAfterAction
	return nil
}

func loadActionsConfig() ActionsConfig {
	data, err := os.ReadFile(actionsPath)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("⚠️  Failed to read actions.json: %v", err)
		}
		return ActionsConfig{Actions: []Action{}}
	}
	var c struct {
		Actions []Action `json:"actions"`
	}
	if err := json.Unmarshal(data, &c); err != nil {
		log.Printf("⚠️  Failed to parse actions.json: %v", err)
		return ActionsConfig{Actions: []Action{}}
	}
	if c.Actions == nil {
		c.Actions = []Action{}
	}
	return ActionsConfig{Actions: c.Actions}
}

func saveActionsConfig(c ActionsConfig) {
	actionsMu.Lock()
	actionsConfig = c
	actionsMu.Unlock()

	dir := filepath.Dir(actionsPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Printf("⚠️  Failed to create actions dir: %v", err)
		return
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		log.Printf("⚠️  Failed to marshal actions.json: %v", err)
		return
	}
	if err := os.WriteFile(actionsPath, data, 0644); err != nil {
		log.Printf("⚠️  Failed to write actions.json: %v", err)
	}
}

func getActions() []Action {
	actionsMu.RLock()
	defer actionsMu.RUnlock()
	out := make([]Action, len(actionsConfig.Actions))
	copy(out, actionsConfig.Actions)
	return out
}

func setActions(actions []Action) {
	saveActionsConfig(ActionsConfig{Actions: actions})
}

func initActionsConfig() {
	actionsConfig = loadActionsConfig()
}
