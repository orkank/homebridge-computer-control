//go:build linux

package main

import (
	"fmt"
	"os"
	"path/filepath"
)

const desktopFileName = "computer-control.desktop"
const systemdServiceName = "computer-control.service"

// getAutostartDir returns the XDG autostart directory.
func getAutostartDir() string {
	configDir := os.Getenv("XDG_CONFIG_HOME")
	if configDir == "" {
		home, _ := os.UserHomeDir()
		configDir = filepath.Join(home, ".config")
	}
	return filepath.Join(configDir, "autostart")
}

func getDesktopFilePath() string {
	return filepath.Join(getAutostartDir(), desktopFileName)
}

// getSystemdUserDir returns the systemd user service directory.
func getSystemdUserDir() string {
	configDir := os.Getenv("XDG_CONFIG_HOME")
	if configDir == "" {
		home, _ := os.UserHomeDir()
		configDir = filepath.Join(home, ".config")
	}
	return filepath.Join(configDir, "systemd", "user")
}

// isAutoStartEnabled checks if the autostart desktop entry exists.
func isAutoStartEnabled() bool {
	_, err := os.Stat(getDesktopFilePath())
	return err == nil
}

// enableAutoStart creates a .desktop file in the XDG autostart directory
// and optionally a systemd user service.
func enableAutoStart() error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot determine executable path: %w", err)
	}

	exePath, err = filepath.Abs(exePath)
	if err != nil {
		return fmt.Errorf("cannot resolve absolute path: %w", err)
	}

	// ── XDG Autostart (.desktop file) ──
	autostartDir := getAutostartDir()
	if err := os.MkdirAll(autostartDir, 0755); err != nil {
		return fmt.Errorf("cannot create autostart dir: %w", err)
	}

	desktopContent := fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=Computer Control
Comment=HomeBridge Computer Control Client
Exec=%s --plugin-url %s --port %d
Hidden=false
NoDisplay=true
X-GNOME-Autostart-enabled=true
StartupNotify=false
Terminal=false
`, exePath, flagPluginURL, flagPort)

	desktopPath := getDesktopFilePath()
	if err := os.WriteFile(desktopPath, []byte(desktopContent), 0644); err != nil {
		return fmt.Errorf("cannot write .desktop file: %w", err)
	}

	// ── Systemd User Service (optional, for headless servers) ──
	serviceDir := getSystemdUserDir()
	_ = os.MkdirAll(serviceDir, 0755)

	serviceContent := fmt.Sprintf(`[Unit]
Description=Computer Control - HomeBridge Client
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=%s --plugin-url %s --port %d
Restart=on-failure
RestartSec=10

[Install]
WantedBy=default.target
`, exePath, flagPluginURL, flagPort)

	servicePath := filepath.Join(serviceDir, systemdServiceName)
	if err := os.WriteFile(servicePath, []byte(serviceContent), 0644); err != nil {
		// Non-fatal: desktop entry is the primary method
		fmt.Printf("⚠️  Could not create systemd service: %v\n", err)
	}

	return nil
}

// disableAutoStart removes the .desktop file and systemd service.
func disableAutoStart() error {
	// Remove .desktop file
	desktopPath := getDesktopFilePath()
	if err := os.Remove(desktopPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("cannot remove .desktop file: %w", err)
	}

	// Remove systemd service
	servicePath := filepath.Join(getSystemdUserDir(), systemdServiceName)
	if err := os.Remove(servicePath); err != nil && !os.IsNotExist(err) {
		// Non-fatal
		fmt.Printf("⚠️  Could not remove systemd service: %v\n", err)
	}

	return nil
}
