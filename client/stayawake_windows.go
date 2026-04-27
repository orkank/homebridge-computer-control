//go:build windows

package main

import (
	"log"
	"sync"
	"syscall"
	"time"
)

var (
	kernel32          = syscall.NewLazyDLL("kernel32.dll")
	procSetThreadExec = kernel32.NewProc("SetThreadExecutionState")
	stayAwakeActive   bool
	stayAwakeMu       sync.Mutex
	stayAwakeStopCh   chan struct{} // signals refresh goroutine to stop
)

const (
	ES_SYSTEM_REQUIRED  = 0x00000001
	ES_DISPLAY_REQUIRED = 0x00000002
	ES_CONTINUOUS       = 0x80000000
)

// refreshInterval: Windows 11 may ignore SetThreadExecutionState; periodic refresh helps.
const stayAwakeRefreshInterval = 60 * time.Second

func startStayAwake() bool {
	stayAwakeMu.Lock()
	defer stayAwakeMu.Unlock()
	if stayAwakeActive {
		return true // already running
	}
	// ES_SYSTEM_REQUIRED: prevent system sleep
	// ES_DISPLAY_REQUIRED: prevent display sleep (screen off)
	// ES_CONTINUOUS: persist until explicitly cleared
	flags := ES_SYSTEM_REQUIRED | ES_DISPLAY_REQUIRED | ES_CONTINUOUS
	r, _, err := procSetThreadExec.Call(uintptr(flags))
	if r == 0 {
		log.Printf("⚠️  SetThreadExecutionState failed: %v", err)
		return false
	}
	stayAwakeActive = true
	// Stop any previous refresh goroutine
	if stayAwakeStopCh != nil {
		close(stayAwakeStopCh)
	}
	stayAwakeStopCh = make(chan struct{})
	go stayAwakeRefreshLoop(stayAwakeStopCh)
	log.Println("☕ Stay-awake ON (SetThreadExecutionState + periodic refresh)")
	if onStayAwakeStateChanged != nil {
		onStayAwakeStateChanged(true)
	}
	return true
}

// stayAwakeRefreshLoop periodically re-applies SetThreadExecutionState.
// Windows 11 can clear the state; PowerToys Awake and others use similar refresh.
func stayAwakeRefreshLoop(stopCh chan struct{}) {
	ticker := time.NewTicker(stayAwakeRefreshInterval)
	defer ticker.Stop()
	for {
		select {
		case <-stopCh:
			return
		case <-ticker.C:
			stayAwakeMu.Lock()
			active := stayAwakeActive
			stayAwakeMu.Unlock()
			if !active {
				return
			}
			flags := ES_SYSTEM_REQUIRED | ES_DISPLAY_REQUIRED | ES_CONTINUOUS
			if r, _, _ := procSetThreadExec.Call(uintptr(flags)); r == 0 {
				log.Printf("⚠️  SetThreadExecutionState (refresh) failed")
			}
		}
	}
}

func stopStayAwake() bool {
	stayAwakeMu.Lock()
	defer stayAwakeMu.Unlock()
	if !stayAwakeActive {
		return true
	}
	// Signal refresh goroutine to stop
	if stayAwakeStopCh != nil {
		close(stayAwakeStopCh)
		stayAwakeStopCh = nil
	}
	// Clear: ES_CONTINUOUS alone resets the state
	r, _, err := procSetThreadExec.Call(uintptr(ES_CONTINUOUS))
	if r == 0 {
		log.Printf("⚠️  SetThreadExecutionState (clear) failed: %v", err)
		return false
	}
	stayAwakeActive = false
	log.Println("☕ Stay-awake OFF")
	if onStayAwakeStateChanged != nil {
		onStayAwakeStateChanged(false)
	}
	return true
}

func isStayAwakeActive() bool {
	stayAwakeMu.Lock()
	defer stayAwakeMu.Unlock()
	return stayAwakeActive
}
