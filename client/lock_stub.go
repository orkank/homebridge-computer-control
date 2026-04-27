//go:build !darwin

package main

import "net/http"

// handleLockDarwin is only used on darwin; this stub satisfies the build.
func handleLockDarwin(http.ResponseWriter, *http.Request) {}
