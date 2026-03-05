//go:build linux

package main

import (
	"log"
	"os/exec"
	"sync"
)

var (
	inhibitCmd *exec.Cmd
	inhibitMu  sync.Mutex
)

func startStayAwake() bool {
	inhibitMu.Lock()
	defer inhibitMu.Unlock()
	if inhibitCmd != nil {
		return true // already running
	}
	// systemd-inhibit prevents sleep while the command runs
	cmd := exec.Command("systemd-inhibit", "--what=sleep", "--who=Computer Control", "--why=Anti-Sleep", "sleep", "infinity")
	if err := cmd.Start(); err != nil {
		log.Printf("⚠️  systemd-inhibit failed: %v", err)
		return false
	}
	inhibitCmd = cmd
	go func() {
		_ = cmd.Wait()
		inhibitMu.Lock()
		if inhibitCmd == cmd {
			inhibitCmd = nil
			if onStayAwakeStateChanged != nil {
				onStayAwakeStateChanged(false)
			}
		}
		inhibitMu.Unlock()
	}()
	log.Println("☕ Stay-awake ON (systemd-inhibit)")
	if onStayAwakeStateChanged != nil {
		onStayAwakeStateChanged(true)
	}
	return true
}

func stopStayAwake() bool {
	inhibitMu.Lock()
	defer inhibitMu.Unlock()
	if inhibitCmd == nil {
		return true
	}
	if err := inhibitCmd.Process.Kill(); err != nil {
		log.Printf("⚠️  Failed to kill systemd-inhibit: %v", err)
		return false
	}
	inhibitCmd = nil
	log.Println("☕ Stay-awake OFF")
	if onStayAwakeStateChanged != nil {
		onStayAwakeStateChanged(false)
	}
	return true
}

func isStayAwakeActive() bool {
	inhibitMu.Lock()
	defer inhibitMu.Unlock()
	return inhibitCmd != nil
}
