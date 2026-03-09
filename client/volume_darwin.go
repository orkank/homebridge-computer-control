//go:build darwin

package main

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

func init() {
	getVolumeLevelImpl = getVolumeLevelDarwin
}

func setVolumeLevel(level int) error {
	level = clamp(level, 0, 100)
	cmd := exec.Command("osascript", "-e", fmt.Sprintf("set volume output volume %d", level))
	return cmd.Run()
}

func getVolumeLevelDarwin() int {
	cmd := exec.Command("osascript", "-e", "output volume of (get volume settings)")
	out, err := cmd.Output()
	if err != nil {
		return -1
	}
	v, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil || v < 0 || v > 100 {
		return -1
	}
	return v
}
