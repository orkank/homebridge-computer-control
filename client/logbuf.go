package main

import (
	"sync"
	"time"
)

const maxLogEntries = 100

var (
	logBuf   []string
	logBufMu sync.Mutex
)

// appendLog adds a line to the GUI log buffer (max 100 entries).
// Safe to call from any goroutine.
func appendLog(msg string) {
	logBufMu.Lock()
	ts := time.Now().Format("15:04:05")
	line := ts + " " + msg
	logBuf = append(logBuf, line)
	if len(logBuf) > maxLogEntries {
		logBuf = logBuf[len(logBuf)-maxLogEntries:]
	}
	logBufMu.Unlock()
	if refreshLogView != nil {
		refreshLogView()
	}
}

// getLogLines returns a copy of the log lines for display.
func getLogLines() []string {
	logBufMu.Lock()
	defer logBufMu.Unlock()
	if len(logBuf) == 0 {
		return nil
	}
	out := make([]string, len(logBuf))
	copy(out, logBuf)
	return out
}

// clearLog clears the log buffer.
func clearLog() {
	logBufMu.Lock()
	logBuf = nil
	logBufMu.Unlock()
	if refreshLogView != nil {
		refreshLogView()
	}
}
