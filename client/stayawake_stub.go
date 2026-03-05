//go:build !darwin && !windows && !linux

package main

func startStayAwake() bool {
	return false
}

func stopStayAwake() bool {
	return true
}

func isStayAwakeActive() bool {
	return false
}
