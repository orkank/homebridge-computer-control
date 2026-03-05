//go:build darwin

package main

import (
	"log"
	"os/exec"
	"sync"
)

var (
	caffeinateCmd *exec.Cmd
	caffeinateMu  sync.Mutex
)

func startStayAwake() bool {
	caffeinateMu.Lock()
	defer caffeinateMu.Unlock()
	if caffeinateCmd != nil {
		return true // already running
	}
	cmd := exec.Command("caffeinate", "-i") // -i = prevent idle sleep
	if err := cmd.Start(); err != nil {
		log.Printf("⚠️  caffeinate -i failed: %v", err)
		return false
	}
	caffeinateCmd = cmd
	go func() {
		_ = cmd.Wait()
		caffeinateMu.Lock()
		if caffeinateCmd == cmd {
			caffeinateCmd = nil
			if onStayAwakeStateChanged != nil {
				onStayAwakeStateChanged(false)
			}
		}
		caffeinateMu.Unlock()
	}()
	log.Println("☕ Stay-awake ON (caffeinate -i)")
	if onStayAwakeStateChanged != nil {
		onStayAwakeStateChanged(true)
	}
	return true
}

func stopStayAwake() bool {
	caffeinateMu.Lock()
	defer caffeinateMu.Unlock()
	if caffeinateCmd == nil {
		return true
	}
	if err := caffeinateCmd.Process.Kill(); err != nil {
		log.Printf("⚠️  Failed to kill caffeinate: %v", err)
		return false
	}
	caffeinateCmd = nil
	log.Println("☕ Stay-awake OFF")
	if onStayAwakeStateChanged != nil {
		onStayAwakeStateChanged(false)
	}
	return true
}

func isStayAwakeActive() bool {
	caffeinateMu.Lock()
	defer caffeinateMu.Unlock()
	return caffeinateCmd != nil
}
