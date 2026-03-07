//go:build !darwin && !linux && !windows

package main

func getCPUTemperatureMillidegree() int {
	return 0
}
