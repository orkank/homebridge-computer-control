//go:build darwin

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"text/template"
)

const launchAgentLabel = "com.homebridge.computer-control"

const plistTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>{{.Label}}</string>
    <key>ProgramArguments</key>
    <array>
        <string>{{.ExePath}}</string>
        <string>--plugin-url</string>
        <string>{{.PluginURL}}</string>
        <string>--port</string>
        <string>{{.Port}}</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <dict>
        <key>NetworkState</key>
        <true/>
    </dict>
    <key>StandardOutPath</key>
    <string>{{.LogPath}}</string>
    <key>StandardErrorPath</key>
    <string>{{.LogPath}}</string>
</dict>
</plist>
`

type plistData struct {
	Label     string
	ExePath   string
	PluginURL string
	Port      string
	LogPath   string
}

func getLaunchAgentPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "LaunchAgents", launchAgentLabel+".plist")
}

func getLogPath() string {
	home, _ := os.UserHomeDir()
	logDir := filepath.Join(home, "Library", "Logs")
	_ = os.MkdirAll(logDir, 0755)
	return filepath.Join(logDir, "computer-control.log")
}

// isAutoStartEnabled checks if the LaunchAgent plist exists.
func isAutoStartEnabled() bool {
	_, err := os.Stat(getLaunchAgentPath())
	return err == nil
}

// enableAutoStart creates a LaunchAgent plist and loads it.
func enableAutoStart() error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot determine executable path: %w", err)
	}

	// Resolve symlinks to get the real path
	exePath, err = filepath.EvalSymlinks(exePath)
	if err != nil {
		return fmt.Errorf("cannot resolve executable path: %w", err)
	}

	data := plistData{
		Label:     launchAgentLabel,
		ExePath:   exePath,
		PluginURL: flagPluginURL,
		Port:      fmt.Sprintf("%d", flagPort),
		LogPath:   getLogPath(),
	}

	// Ensure LaunchAgents directory exists
	dir := filepath.Dir(getLaunchAgentPath())
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("cannot create LaunchAgents dir: %w", err)
	}

	// Write plist file
	f, err := os.Create(getLaunchAgentPath())
	if err != nil {
		return fmt.Errorf("cannot create plist: %w", err)
	}
	defer f.Close()

	tmpl, err := template.New("plist").Parse(plistTemplate)
	if err != nil {
		return fmt.Errorf("template parse error: %w", err)
	}

	if err := tmpl.Execute(f, data); err != nil {
		return fmt.Errorf("template execute error: %w", err)
	}

	// Do NOT run "launchctl load" — RunAtLoad:true would start a second instance immediately.
	// The plist will be auto-loaded by launchd at next user login.
	return nil
}

// disableAutoStart unloads and removes the LaunchAgent plist.
func disableAutoStart() error {
	plistPath := getLaunchAgentPath()

	// Unload first (ignore errors if not loaded)
	cmd := exec.Command("launchctl", "unload", plistPath)
	_ = cmd.Run()

	// Remove the plist file
	if err := os.Remove(plistPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("cannot remove plist: %w", err)
	}

	return nil
}
