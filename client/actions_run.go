package main

import (
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// bttcliFullPath is the default location of bttcli on macOS.
// GUI apps don't inherit shell PATH (.zprofile etc.), so we use full path.
const bttcliFullPath = "/Applications/BetterTouchTool.app/Contents/SharedSupport/bin/bttcli"

// runAction executes an action by type. Fire-and-forget: returns 200 OK immediately.
// Variable {status} in Value is replaced with "on" or "off".
func runAction(a Action, state string) {
	val := strings.ReplaceAll(a.Value, "{status}", state)
	val = strings.TrimSpace(val)
	if val == "" {
		log.Printf("⚠️  Action %q has empty value after substitution", a.Name)
		return
	}

	switch a.Type {
	case ActionTypeBTTTrigger:
		runBTTCommand(val)
	case ActionTypeShell:
		runShell(val)
	case ActionTypeBatch:
		runBatch(val)
	case ActionTypeAppleScript:
		runAppleScript(val)
	case ActionTypeURL:
		runURLAction(a, val)
	default:
		log.Printf("⚠️  Unknown action type: %s", a.Type)
	}
}

// stripBTTCLIPrefix removes leading "bttcli" from user input so we don't double-prefix.
func stripBTTCLIPrefix(val string) string {
	val = strings.TrimSpace(val)
	if strings.HasPrefix(strings.ToLower(val), "bttcli") {
		val = strings.TrimSpace(val[6:])
	}
	return val
}

// parseShellArgs splits a shell-style string into arguments (respects double quotes).
// E.g. `trigger_named "mute"` → ["trigger_named", "mute"]
func parseShellArgs(s string) []string {
	var args []string
	var current strings.Builder
	inQuotes := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '"' {
			inQuotes = !inQuotes
		} else if (c == ' ' || c == '\t') && !inQuotes {
			if current.Len() > 0 {
				args = append(args, current.String())
				current.Reset()
			}
		} else {
			current.WriteByte(c)
		}
	}
	if current.Len() > 0 {
		args = append(args, current.String())
	}
	return args
}

// runBTTCommand runs the full BTT CLI command. User writes whatever comes after bttcli.
// Examples: trigger_named "mute", display_notification "Hi" title="Title", get_clipboard_content
// Uses full path to bttcli and passes args directly (no shell) so bttcli receives correct argv.
func runBTTCommand(val string) {
	val = stripBTTCLIPrefix(val)
	if val == "" {
		log.Printf("⚠️  BTT command is empty")
		return
	}
	if runtime.GOOS != "darwin" {
		log.Printf("⚠️  BTT is macOS-only")
		return
	}
	bttcli := bttcliFullPath
	if _, err := os.Stat(bttcli); os.IsNotExist(err) {
		bttcli = "bttcli"
	}
	args := parseShellArgs(val)
	if len(args) == 0 {
		log.Printf("⚠️  BTT command is empty after parsing")
		return
	}
	cmd := exec.Command(bttcli, args...)
	prepareCmd(cmd)
	if err := cmd.Start(); err != nil {
		log.Printf("⚠️  BTT command failed: %v", err)
		return
	}
	go func() { _ = cmd.Wait() }()
	log.Printf("🎯 BTT: %s", val)
}

// runShell runs a shell command in background (sh -c on Unix, cmd /c on Windows).
func runShell(val string) {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/c", val)
	} else {
		cmd = exec.Command("sh", "-c", val)
	}
	prepareCmd(cmd)
	if err := cmd.Start(); err != nil {
		log.Printf("⚠️  Shell command failed: %v", err)
		return
	}
	go func() { _ = cmd.Wait() }()
	log.Printf("🔧 Shell: %s", val)
}

// runBatch runs a .bat/.cmd file on Windows; on Unix treats as shell script.
func runBatch(val string) {
	if runtime.GOOS == "windows" {
		cmd := exec.Command("cmd", "/c", val)
		prepareCmd(cmd)
		if err := cmd.Start(); err != nil {
			log.Printf("⚠️  Batch failed: %v", err)
			return
		}
		go func() { _ = cmd.Wait() }()
	} else {
		runShell(val)
	}
	log.Printf("📜 Batch: %s", val)
}

// runAppleScript runs AppleScript (macOS only).
// If val is a path to an existing file (.applescript, .scpt), runs: osascript <path>
// Otherwise runs inline: osascript -e "val"
func runAppleScript(val string) {
	if runtime.GOOS != "darwin" {
		log.Printf("⚠️  AppleScript is macOS-only")
		return
	}
	var cmd *exec.Cmd
	if absPath := resolveAppleScriptPath(val); absPath != "" {
		cmd = exec.Command("osascript", absPath)
	} else {
		cmd = exec.Command("osascript", "-e", val)
	}
	prepareCmd(cmd)
	if err := cmd.Start(); err != nil {
		log.Printf("⚠️  AppleScript failed: %v", err)
		return
	}
	go func() { _ = cmd.Wait() }()
	log.Printf("🍎 AppleScript: %s", val)
}

func expandTilde(s string) string {
	if s == "~" || strings.HasPrefix(s, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return s
		}
		if s == "~" {
			return home
		}
		return home + s[1:]
	}
	return s
}

// resolveAppleScriptPath returns the absolute path if val is an existing .applescript/.scpt file, else "".
func resolveAppleScriptPath(s string) string {
	p := expandTilde(strings.TrimSpace(s))
	if p == "" {
		return ""
	}
	lower := strings.ToLower(p)
	if !strings.HasSuffix(lower, ".applescript") && !strings.HasSuffix(lower, ".scpt") {
		return ""
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		abs = p
	}
	info, err := os.Stat(abs)
	if err != nil || info.IsDir() {
		return ""
	}
	return abs
}

// runURLAction runs URL based on URLMode: fetch (HTTP GET) or browser (open in default browser).
func runURLAction(a Action, url string) {
	mode := a.URLMode
	if mode != URLModeBrowser && mode != URLModeFetch {
		mode = URLModeFetch
	}
	if mode == URLModeBrowser {
		if err := openURLInBrowser(url); err != nil {
			log.Printf("⚠️  Open URL in browser failed: %v", err)
			return
		}
		log.Printf("🌐 Opened in browser: %s", url)
		return
	}
	// URLModeFetch: HTTP GET in background
	go func() {
		resp, err := http.Get(url)
		if err != nil {
			log.Printf("⚠️  URL request failed: %v", err)
			return
		}
		resp.Body.Close()
		log.Printf("🌐 URL: %s → %d", url, resp.StatusCode)
	}()
}
