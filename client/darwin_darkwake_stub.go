//go:build !darwin

package main

func isDisplayInDarkWake() bool {
	return false
}

func getDisplayStateInfo() DisplayStateInfo {
	return DisplayStateInfo{}
}
