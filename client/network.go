package main

import (
	"fmt"
	"net"
	"os/exec"
	"runtime"
	"strings"
)

// isVirtualInterface returns true if the interface name suggests a virtual adapter
// (Hyper-V, WSL, Docker, VMware, VirtualBox, etc.) which often has a wrong IP.
func isVirtualInterface(name string) bool {
	name = strings.ToLower(name)
	virtual := []string{
		"vethernet", "hyper-v", "vmware", "virtualbox", "docker",
		"wsl", "virtual", "loopback", "teredo", "isatap",
	}
	for _, v := range virtual {
		if strings.Contains(name, v) {
			return true
		}
	}
	return false
}

// isLikelyPhysicalIP returns true for common "real" network ranges (home/office).
// 172.16.0.0/12 is often used by virtual adapters (Hyper-V, Docker, etc.).
func isLikelyPhysicalIP(ip net.IP) bool {
	if ip == nil || ip.To4() == nil {
		return false
	}
	// Prefer 192.168.x.x and 10.x.x.x over 172.16-31.x.x
	octets := ip.To4()
	if octets[0] == 192 && octets[1] == 168 {
		return true
	}
	if octets[0] == 10 {
		return true
	}
	// 172.16.0.0/12 - often virtual, but could be real
	return false
}

// getNetworkInfo returns the primary non-loopback IPv4 address and its MAC.
// On Windows, skips virtual adapters (Hyper-V, WSL, Docker) that report wrong IPs.
func getNetworkInfo() (string, string, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return "", "", fmt.Errorf("failed to list interfaces: %w", err)
	}

	var bestIP, bestMAC string
	var bestScore int

	for _, iface := range interfaces {
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		if iface.Flags&net.FlagUp == 0 {
			continue
		}
		if len(iface.HardwareAddr) == 0 {
			continue
		}
		if runtime.GOOS == "windows" && isVirtualInterface(iface.Name) {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			if ip == nil || ip.IsLoopback() || ip.To4() == nil {
				continue
			}

			score := 0
			if isLikelyPhysicalIP(ip) {
				score = 2 // Prefer 192.168.x.x, 10.x.x.x
			} else {
				score = 1 // 172.x.x.x etc.
			}

			if score > bestScore || (score == bestScore && bestIP == "") {
				bestScore = score
				bestIP = ip.String()
				bestMAC = iface.HardwareAddr.String()
			}
		}
	}

	if bestIP != "" {
		return bestIP, bestMAC, nil
	}

	return "", "", fmt.Errorf("no suitable network interface found")
}

// getHostname returns the machine hostname.
func getHostname() string {
	out, err := exec.Command("hostname").Output()
	if err != nil {
		return "Unknown"
	}
	return strings.TrimSpace(string(out))
}

// getOSDisplayName returns a human-readable OS name.
func getOSDisplayName() string {
	switch runtime.GOOS {
	case "darwin":
		return "macOS"
	case "windows":
		return "Windows"
	case "linux":
		return "Linux"
	default:
		return runtime.GOOS
	}
}
