//go:build windows

package main

import (
	"log"
	"sync"
	"syscall"
)

var (
	kernel32           = syscall.NewLazyDLL("kernel32.dll")
	procSetThreadExec  = kernel32.NewProc("SetThreadExecutionState")
	stayAwakeActive    bool
	stayAwakeMu        sync.Mutex
)

const (
	ES_SYSTEM_REQUIRED = 0x00000001
	ES_CONTINUOUS      = 0x80000000
)

func startStayAwake() bool {
	stayAwakeMu.Lock()
	defer stayAwakeMu.Unlock()
	r, _, err := procSetThreadExec.Call(uintptr(ES_SYSTEM_REQUIRED | ES_CONTINUOUS))
	if r == 0 {
		log.Printf("⚠️  SetThreadExecutionState failed: %v", err)
		return false
	}
	stayAwakeActive = true
	log.Println("☕ Stay-awake ON (SetThreadExecutionState)")
	if onStayAwakeStateChanged != nil {
		onStayAwakeStateChanged(true)
	}
	return true
}

func stopStayAwake() bool {
	stayAwakeMu.Lock()
	defer stayAwakeMu.Unlock()
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
