//go:build darwin && cgo

package main

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa -framework Foundation
#import <Cocoa/Cocoa.h>

void hideDockIcon() {
	[[NSApplication sharedApplication] setActivationPolicy:NSApplicationActivationPolicyAccessory];
}
*/
import "C"

// hideDockIconMacOS hides the app icon from the macOS Dock.
// Call this after the Fyne app is created (from main).
func hideDockIconMacOS() {
	C.hideDockIcon()
}
