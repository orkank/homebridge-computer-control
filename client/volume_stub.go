//go:build !darwin && !windows && !linux

package main

import "fmt"

func setVolumeLevel(level int) error {
	return fmt.Errorf("volume control not supported on this platform")
}
