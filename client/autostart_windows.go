//go:build windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
)

const startupScriptName = "ComputerControl.bat"

// getStartupFolderPath returns the Windows Startup folder path.
func getStartupFolderPath() string {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		home, _ := os.UserHomeDir()
		appData = filepath.Join(home, "AppData", "Roaming")
	}
	return filepath.Join(appData, "Microsoft", "Windows", "Start Menu", "Programs", "Startup")
}

func getStartupScriptPath() string {
	return filepath.Join(getStartupFolderPath(), startupScriptName)
}

// isAutoStartEnabled checks if the startup script exists.
func isAutoStartEnabled() bool {
	_, err := os.Stat(getStartupScriptPath())
	return err == nil
}

// enableAutoStart creates a batch file in the Windows Startup folder.
// The exe was compiled with -H windowsgui so no console window appears.
func enableAutoStart() error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot determine executable path: %w", err)
	}

	exePath, err = filepath.Abs(exePath)
	if err != nil {
		return fmt.Errorf("cannot resolve absolute path: %w", err)
	}

	// Create a VBS wrapper that launches the exe without any window
	vbsContent := fmt.Sprintf(
		`Set WshShell = CreateObject("WScript.Shell")`+"\r\n"+
			`WshShell.Run """%s"" --plugin-url %s --port %d", 0, False`+"\r\n"+
			`Set WshShell = Nothing`+"\r\n",
		exePath, flagPluginURL, flagPort,
	)

	// Use .vbs instead of .bat for truly silent startup
	vbsPath := filepath.Join(getStartupFolderPath(), "ComputerControl.vbs")
	if err := os.WriteFile(vbsPath, []byte(vbsContent), 0644); err != nil {
		return fmt.Errorf("cannot write startup script: %w", err)
	}

	return nil
}

// disableAutoStart removes the startup script.
func disableAutoStart() error {
	// Remove .vbs
	vbsPath := filepath.Join(getStartupFolderPath(), "ComputerControl.vbs")
	if err := os.Remove(vbsPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("cannot remove startup script: %w", err)
	}

	// Also clean up any old .bat version
	batPath := getStartupScriptPath()
	if err := os.Remove(batPath); err != nil && !os.IsNotExist(err) {
		// Ignore — may not exist
	}

	return nil
}
