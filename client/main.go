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
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
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
	statusLabel    *widget.RichText
	ipLabel       *widget.Label
	stayAwakeLabel *widget.Label
	// onStayAwakeStateChanged is set by buildMainUI; stayawake_*.go calls it when state changes
	onStayAwakeStateChanged func(bool)
	// refreshLogView is set by buildMainUI; logbuf.go calls it when new log entries arrive
	refreshLogView func()

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
	hideDockIconMacOS() // no-op on non-darwin; hides dock icon on macOS

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
	mainWindow.Resize(fyne.NewSize(420, 480))
	mainWindow.SetFixedSize(true)
	mainWindow.CenterOnScreen()

	content := buildMainUI()
	mainWindow.SetContent(content)

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

// ──────────────────────────────────────────────
// GUI Construction
// ──────────────────────────────────────────────

func buildMainUI() fyne.CanvasObject {
	// ── Header with icon ──
	headerIcon := canvas.NewImageFromResource(getAppIcon())
	headerIcon.SetMinSize(fyne.NewSize(48, 48))
	headerIcon.FillMode = canvas.ImageFillContain

	headerTitle := widget.NewRichTextFromMarkdown("## 🖥 Computer Control v" + clientVersion)

	header := container.NewHBox(
		headerIcon,
		layout.NewSpacer(),
		headerTitle,
		layout.NewSpacer(),
	)

	// ── Info card ──
	hostEntry := widget.NewEntry()
	hostEntry.SetText(appState.Hostname)
	hostEntry.OnChanged = func(val string) {
		appState.Hostname = val
		fyneApp.Preferences().SetString("hostname", val)
	}

	ipLabel = widget.NewLabel(appState.IP)
	ipLabel.TextStyle = fyne.TextStyle{Monospace: true}

	macValue := widget.NewLabel(appState.MAC)
	macValue.TextStyle = fyne.TextStyle{Monospace: true}

	osValue := widget.NewLabel(appState.OS)

	statusLabel = widget.NewRichTextFromMarkdown("⏳ **Waiting...**")
	stayAwakeLabel = widget.NewLabel("")
	stayAwakeLabel.TextStyle = fyne.TextStyle{Bold: true}

	onStayAwakeStateChanged = func(active bool) {
		fyne.Do(func() {
			if stayAwakeLabel != nil {
				if active {
					stayAwakeLabel.SetText("☕ Anti-Sleep: ON")
				} else {
					stayAwakeLabel.SetText("")
				}
				stayAwakeLabel.Refresh()
			}
		})
	}
	// Show current state if already active (e.g. after restart with plugin having it ON)
	if isStayAwakeActive() {
		stayAwakeLabel.SetText("☕ Anti-Sleep: ON")
	}

	infoForm := widget.NewForm(
		widget.NewFormItem("Version", widget.NewLabel(clientVersion)),
		widget.NewFormItem("Computer Name", hostEntry),
		widget.NewFormItem("Local IP", ipLabel),
		widget.NewFormItem("MAC Address", macValue),
		widget.NewFormItem("Operating System", osValue),
		widget.NewFormItem("Listening Port", widget.NewLabel(fmt.Sprintf("%d", flagPort))),
		widget.NewFormItem("Status", statusLabel),
		widget.NewFormItem("Anti-Sleep", stayAwakeLabel),
	)

	infoCard := widget.NewCard("", "Device Information", infoForm)

	// ── Plugin URL entry ──
	pluginEntry := widget.NewEntry()
	pluginEntry.SetText(flagPluginURL)
	pluginEntry.SetPlaceHolder("http://homebridge-ip:9090")
	pluginEntry.OnChanged = func(val string) {
		flagPluginURL = val
		fyneApp.Preferences().SetString("plugin-url", val)
	}

	testBtn := widget.NewButton("Test", func() {
		statusLabel.ParseMarkdown("⏳ **Testing...**")
		statusLabel.Refresh()
		go func() {
			ok := sendHeartbeat(appState.Hostname, appState.IP, appState.MAC)
			updateConnectionStatus(ok)
		}()
	})
	pluginBox := container.NewBorder(nil, nil, nil, testBtn, pluginEntry)

	pluginCard := widget.NewCard("", "Homebridge Plugin URL", pluginBox)

	// ── Log viewer (heartbeat, update, errors) ──
	logEntry := widget.NewMultiLineEntry()
	logEntry.SetMinRowsVisible(4)
	logEntry.Disable()
	logEntry.Wrapping = fyne.TextWrapWord

	refreshLogView = func() {
		fyne.Do(func() {
			if logEntry != nil {
				lines := getLogLines()
				var text string
				for _, l := range lines {
					if text != "" {
						text += "\n"
					}
					text += l
				}
				logEntry.SetText(text)
				if len(lines) > 0 {
					logEntry.CursorRow = len(lines) - 1
				}
				logEntry.Refresh()
			}
		})
	}

	clearLogBtn := widget.NewButton("Clear", func() {
		clearLog()
	})
	logToolbar := container.NewHBox(layout.NewSpacer(), clearLogBtn)
	logCard := widget.NewCard("", "Log", container.NewBorder(nil, logToolbar, nil, nil, logEntry))

	// ── Auto-start checkbox ──
	autoStartCheck := widget.NewCheck("Run at Startup (Auto-Start)", func(checked bool) {
		var asErr error
		if checked {
			asErr = enableAutoStart()
		} else {
			asErr = disableAutoStart()
		}
		if asErr != nil {
			log.Printf("⚠️  Auto-start error: %v", asErr)
			dialog.ShowError(fmt.Errorf("Failed to set auto-start: %v", asErr), mainWindow)
		}
	})
	autoStartCheck.Checked = isAutoStartEnabled()

	// ── Buttons ──
	hideBtn := widget.NewButtonWithIcon("Hide", theme.VisibilityOffIcon(), func() {
		mainWindow.Hide()
	})
	hideBtn.Importance = widget.MediumImportance

	exitBtn := widget.NewButtonWithIcon("Exit", theme.CancelIcon(), func() {
		fyneApp.Quit()
	})
	exitBtn.Importance = widget.DangerImportance

	buttonBar := container.NewHBox(
		layout.NewSpacer(),
		hideBtn,
		exitBtn,
		layout.NewSpacer(),
	)

	// ── Assemble ──
	return container.NewVBox(
		header,
		widget.NewSeparator(),
		infoCard,
		pluginCard,
		logCard,
		autoStartCheck,
		layout.NewSpacer(),
		widget.NewSeparator(),
		buttonBar,
	)
}

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
	if statusLabel == nil {
		return
	}

	if connected {
		statusLabel.ParseMarkdown("🟢 **Connected**")
	} else {
		statusLabel.ParseMarkdown("🔴 **Disconnected**")
	}
	statusLabel.Refresh()
}

// updateIPDisplay refreshes the IP label if it changed.
func updateIPDisplay(newIP string) {
	appState.IP = newIP
	if ipLabel == nil {
		return
	}
	ipLabel.SetText(newIP)
}
