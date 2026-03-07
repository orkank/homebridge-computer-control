package main

import (
	"flag"
	"fmt"
	"log"
	"runtime"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

// AppState holds the live application state shared between GUI and services.
type AppState struct {
	Hostname  string
	IP        string
	MAC       string
	OS        string
	Connected bool
}

var (
	flagPluginURL string
	flagPort      int
	startTime     time.Time

	// Mutable labels updated from background goroutines
	statusDot     *statusDotWidget
	ipLabel       *widget.Label
	stayAwakeLabel *widget.Label
	// onStayAwakeStateChanged is set by buildMainUI; stayawake_*.go calls it when state changes
	onStayAwakeStateChanged func(bool)
	// refreshLogView is set by buildMainUI; logbuf.go calls it when new log entries arrive
	refreshLogView func()
	// onHeartbeatSending is called when heartbeat starts/stops; used for status dot (orange)
	onHeartbeatSending func(bool)

	// The main Fyne app & window
	fyneApp    fyne.App
	mainWindow fyne.Window

	// Shared state
	appState AppState

	// One-time update notification (shown at most once per session)
	updateNotificationOnce sync.Once
)

func main() {
	flag.StringVar(&flagPluginURL, "plugin-url", "", "Homebridge plugin registration URL (e.g. http://192.168.1.10:9090)")
	flag.IntVar(&flagPort, "port", 45991, "HTTP server port")
	showVersion := flag.Bool("version", false, "Print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("Computer Control %s\n", clientVersion)
		return
	}

	startTime = time.Now()

	// ── Create Fyne App ──
	fyneApp = app.NewWithID("com.homebridge.computer-control")
	fyneApp.SetIcon(getAppIcon())
	fyneApp.Settings().SetTheme(newMacTheme()) // macOS-style light/dark, accent
	hideDockIconMacOS()                             // no-op on non-darwin; hides dock icon on macOS

	// ── Load client config (client_config.json) ──
	initClientConfig()

	// ── Load saved preferences ──
	if flagPluginURL == "" {
		flagPluginURL = fyneApp.Preferences().StringWithFallback("plugin-url", "http://localhost:9090")
	} else {
		fyneApp.Preferences().SetString("plugin-url", flagPluginURL)
	}

	const defaultPort = 45991
	const oldDefaultPort = 8080
	savedPort := fyneApp.Preferences().IntWithFallback("port", defaultPort)
	// Migrate: old default 8080 → new default 45991 (avoids port conflicts)
	if savedPort == oldDefaultPort {
		savedPort = defaultPort
		fyneApp.Preferences().SetInt("port", defaultPort)
	}
	if flagPort == defaultPort {
		flagPort = savedPort
	}
	fyneApp.Preferences().SetInt("port", flagPort)

	// ── Detect network ──
	ip, mac, err := getNetworkInfo()
	if err != nil {
		log.Printf("⚠️  Network detection failed: %v", err)
		ip = "Not detected"
		mac = "Not detected"
	}
	hostname := fyneApp.Preferences().StringWithFallback("hostname", getHostname())

	appState = AppState{
		Hostname:  hostname,
		IP:        ip,
		MAC:       mac,
		OS:        getOSDisplayName(),
		Connected: false,
	}

	log.Println("🖥️  Computer Control starting...")
	log.Printf("   Hostname: %s", hostname)
	log.Printf("   IP: %s | MAC: %s", ip, mac)
	log.Printf("   Port: %d", flagPort)
	log.Printf("   Plugin: %s", flagPluginURL)

	// ── Build GUI ──
	mainWindow = fyneApp.NewWindow("Computer Control")
	mainWindow.Resize(fyne.NewSize(520, 560))
	mainWindow.CenterOnScreen()

	content := buildMainUI()
	mainWindow.SetContent(content)

	// ── Theme change listener (light/dark) — refresh theme-dependent widgets ──
	fyneApp.Settings().AddListener(func(_ fyne.Settings) {
		fyne.Do(func() {
			for _, w := range themeDependentWidgets {
				if w != nil {
					w.Refresh()
				}
			}
		})
	})

	// ── Window close → hide to tray ──
	mainWindow.SetCloseIntercept(func() {
		mainWindow.Hide()
	})

	// Add lifecycle hook to ensure window is completely hidden from OS window manager
	fyneApp.Lifecycle().SetOnEnteredForeground(func() {
		mainWindow.Show()
	})

	// ── Auth token (from plugin registration; required for sleep/health/wake-screen) ──
	getAuthToken = func() string {
		return fyneApp.Preferences().StringWithFallback("auth-token", "")
	}
	setAuthToken = func(token string) {
		fyneApp.Preferences().SetString("auth-token", token)
	}

	// ── Update notification (one-time per session; opens download link on user action) ──
	onUpdateAvailable = func(downloadURL string) {
		showUpdateNotification(downloadURL)
	}

	// ── System Tray ──
	setupSystemTray()

	// ── Start background services ──
	go startHTTPServer(hostname)
	go heartbeatLoop()
	runAutoDisplayWake() // macOS: caffeinate on startup to force display wake after boot/wake

	// ── Show and run ──
	mainWindow.ShowAndRun()
}

// buildMainUI is in ui_layout.go

// ──────────────────────────────────────────────
// System Tray
// ──────────────────────────────────────────────

func setupSystemTray() {
	if desk, ok := fyneApp.(desktop.App); ok {
		trayMenu := fyne.NewMenu("Computer Control",
			fyne.NewMenuItem("Open", func() {
				mainWindow.Show()
				mainWindow.RequestFocus()
			}),
		)

		desk.SetSystemTrayMenu(trayMenu)
		desk.SetSystemTrayIcon(getAppIcon())
		desk.SetSystemTrayWindow(mainWindow) // Left-click tray icon → open main window directly
	}
}

// showUpdateNotification displays a one-time dialog when an update is available.
// User can click "Download" to open the download URL in browser.
func showUpdateNotification(downloadURL string) {
	updateNotificationOnce.Do(func() {
		fyne.Do(func() {
			mainWindow.Show()
			mainWindow.RequestFocus()

			msg := widget.NewLabel("A new version is available. Click Download to open the download page.")
			msg.Wrapping = fyne.TextWrapWord

			content := container.NewVBox(msg)
			if runtime.GOOS == "darwin" {
				macHint := widget.NewLabel("Mac: If macOS says \"damaged\", right-click the app → Open, or run: xattr -cr ComputerControl.app")
				macHint.Wrapping = fyne.TextWrapWord
				content.Add(macHint)
			}
			content.Add(widget.NewButton("Download", func() {
				if err := openURLInBrowser(downloadURL); err != nil {
					log.Printf("⚠️  Failed to open URL: %v", err)
					dialog.ShowError(fmt.Errorf("Could not open browser: %v", err), mainWindow)
				}
			}))

			dlg := dialog.NewCustom("Update Available", "Close", content, mainWindow)
			dlg.Show()
		})
	})
}

// ──────────────────────────────────────────────
// GUI State Updates (thread-safe via Fyne)
// ──────────────────────────────────────────────

// updateConnectionStatus is called from background goroutines.
func updateConnectionStatus(connected bool) {
	appState.Connected = connected
	fyne.Do(func() {
		if statusDot == nil {
			return
		}
		if connected {
			statusDot.SetState(StatusDotOnline)
		} else {
			statusDot.SetState(StatusDotError)
		}
	})
}

// updateIPDisplay refreshes the IP label if it changed.
func updateIPDisplay(newIP string) {
	appState.IP = newIP
	fyne.Do(func() {
		if ipLabel != nil {
			ipLabel.SetText(newIP)
		}
	})
}
