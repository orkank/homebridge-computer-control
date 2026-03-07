package main

import (
	"io"
	"log"
	"os"
	"strings"
	"sync"
	"time"
)

const maxLogEntries = 100

var (
	logBuf   []string
	logBufMu sync.Mutex
)

// guiLogWriter is an io.Writer that sends log output to both os.Stderr and the GUI log buffer.
type guiLogWriter struct {
	stderr io.Writer
}

func (w *guiLogWriter) Write(p []byte) (n int, err error) {
	// Always write to stderr (terminal)
	n, err = w.stderr.Write(p)
	// Also push to GUI log buffer (strip trailing newline)
	msg := strings.TrimRight(string(p), "\n")
	if msg != "" {
		appendLogRaw(msg)
	}
	return n, err
}

func init() {
	// Redirect Go's standard log to write to both stderr and the GUI log buffer
	log.SetOutput(&guiLogWriter{stderr: os.Stderr})
	log.SetFlags(0) // we add our own timestamp in appendLog/appendLogRaw
}


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

// appendLogRaw adds a pre-formatted line (with its own timestamp) to the GUI log buffer.
// Used by guiLogWriter to capture standard log output.
func appendLogRaw(msg string) {
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
