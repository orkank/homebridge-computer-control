//go:build !darwin || !cgo

package main

// hideDockIconMacOS is a no-op on non-macOS platforms.
func hideDockIconMacOS() {}
