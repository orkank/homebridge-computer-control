//go:build linux

package main

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

func init() {
	getVolumeLevelImpl = getVolumeLevelLinux
}

func setVolumeLevel(level int) error {
	level = clamp(level, 0, 100)
	// Try pactl first (PulseAudio), then amixer (ALSA)
	cmd := exec.Command("pactl", "set-sink-volume", "@DEFAULT_SINK@", fmt.Sprintf("%d%%", level))
	if err := cmd.Run(); err != nil {
		cmd = exec.Command("amixer", "-D", "pulse", "set", "Master", fmt.Sprintf("%d%%", level))
		if err := cmd.Run(); err != nil {
			cmd = exec.Command("amixer", "set", "Master", fmt.Sprintf("%d%%", level))
			return cmd.Run()
		}
	}
	return nil
}

func getVolumeLevelLinux() int {
	// Try pactl first
	cmd := exec.Command("pactl", "get-sink-volume", "@DEFAULT_SINK@")
	out, err := cmd.Output()
	if err == nil {
		// Output: "Volume: front-left: 65536 / 65536 / 0 dB, ..." or "Volume: front-left: 32768 / 65536 / 0 dB"
		// Parse percentage
		lines := strings.Split(string(out), "\n")
		for _, line := range lines {
			if strings.Contains(line, "/") {
				parts := strings.Split(line, "/")
				if len(parts) >= 2 {
					var current, max int
					fmt.Sscanf(strings.TrimSpace(parts[0]), "Volume: front-left: %d", &current)
					fmt.Sscanf(strings.TrimSpace(parts[1]), "%d", &max)
					if max > 0 {
						return clamp((current*100)/max, 0, 100)
					}
				}
			}
		}
	}
	// Fallback: amixer
	cmd = exec.Command("amixer", "-D", "pulse", "get", "Master")
	out, err = cmd.Output()
	if err != nil {
		cmd = exec.Command("amixer", "get", "Master")
		out, err = cmd.Output()
	}
	if err != nil {
		return -1
	}
	// Parse "Playback 50 [50%]"
	for _, line := range strings.Split(string(out), "\n") {
		if idx := strings.Index(line, "%"); idx > 0 {
			sub := line[:idx]
			for i := len(sub) - 1; i >= 0; i-- {
				if sub[i] >= '0' && sub[i] <= '9' {
					end := i + 1
					for i > 0 && sub[i-1] >= '0' && sub[i-1] <= '9' {
						i--
					}
					v, _ := strconv.Atoi(sub[i:end])
					return clamp(v, 0, 100)
				}
			}
		}
	}
	return -1
}
