package main

import (
	"fmt"
	"image/color"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// themeAwareRect is a rectangle that updates its color when theme changes.
// Used for sidebar, separators, etc. that must follow light/dark mode.
type themeAwareRect struct {
	widget.BaseWidget
	lightColor color.Color
	darkColor  color.Color
	rect       *canvas.Rectangle
	minSize    fyne.Size
}

func newThemeAwareRect(light, dark color.Color) *themeAwareRect {
	t := &themeAwareRect{lightColor: light, darkColor: dark}
	t.ExtendBaseWidget(t)
	return t
}

func (t *themeAwareRect) SetMinSize(s fyne.Size) {
	t.minSize = s
}

func (t *themeAwareRect) CreateRenderer() fyne.WidgetRenderer {
	c := t.lightColor
	if fyneApp != nil && fyneApp.Settings().ThemeVariant() == theme.VariantDark {
		c = t.darkColor
	}
	t.rect = canvas.NewRectangle(c)
	return &themeAwareRectRenderer{t: t}
}

type themeAwareRectRenderer struct {
	t *themeAwareRect
}

func (r *themeAwareRectRenderer) Layout(size fyne.Size) {
	r.t.rect.Resize(size)
}

func (r *themeAwareRectRenderer) MinSize() fyne.Size {
	if r.t.minSize != (fyne.Size{}) {
		return r.t.minSize
	}
	return r.t.rect.MinSize()
}

func (r *themeAwareRectRenderer) Refresh() {
	if fyneApp != nil && fyneApp.Settings().ThemeVariant() == theme.VariantDark {
		r.t.rect.FillColor = r.t.darkColor
	} else {
		r.t.rect.FillColor = r.t.lightColor
	}
	r.t.rect.Refresh()
}

func (r *themeAwareRectRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.t.rect}
}

func (r *themeAwareRectRenderer) Destroy() {}

// ──────────────────────────────────────────────
// Helper – "secondary" colored label (grey for labels)
// ──────────────────────────────────────────────

func secondaryLabel(text string) *widget.RichText {
	rt := widget.NewRichTextWithText(text)
	seg := rt.Segments[0].(*widget.TextSegment)
	seg.Style.ColorName = theme.ColorNameDisabled
	return rt
}

// ──────────────────────────────────────────────
// Main UI
// ──────────────────────────────────────────────

// themeDependentWidgets holds widgets that must refresh when theme (light/dark) changes.
var themeDependentWidgets []fyne.Widget

// buildMainUI constructs the full UI with an Apple-style sidebar navigation.
func buildMainUI() fyne.CanvasObject {
	themeDependentWidgets = nil

	// ── Content area (swappable) ──
	contentArea := container.NewMax()

	dashboardContent := buildDashboardContent()
	settingsContent := buildSettingsContent()
	logsContent := buildLogsContent()

	// ── Sidebar items ──
	var dashItem, settItem, logItem *sidebarItem
	selectItem := func(which *sidebarItem) {
		dashItem.SetSelected(which == dashItem)
		settItem.SetSelected(which == settItem)
		logItem.SetSelected(which == logItem)
	}

	dashItem = newSidebarItem("Dashboard", theme.HomeIcon(), func() {
		selectItem(dashItem)
		contentArea.Objects = []fyne.CanvasObject{dashboardContent}
		contentArea.Refresh()
	})
	settItem = newSidebarItem("Settings", theme.SettingsIcon(), func() {
		selectItem(settItem)
		contentArea.Objects = []fyne.CanvasObject{settingsContent}
		contentArea.Refresh()
	})
	logItem = newSidebarItem("Logs", theme.DocumentIcon(), func() {
		selectItem(logItem)
		contentArea.Objects = []fyne.CanvasObject{logsContent}
		contentArea.Refresh()
	})

	// Default selection
	dashItem.SetSelected(true)
	contentArea.Objects = []fyne.CanvasObject{dashboardContent}

	sidebarList := container.NewVBox(
		layout.NewSpacer(),
		dashItem,
		settItem,
		logItem,
		layout.NewSpacer(),
	)

	// Sidebar background (theme-aware: light/dark)
	sidebarBg := newThemeAwareRect(macLightSidebar, macDarkSidebar)
	themeDependentWidgets = append(themeDependentWidgets, sidebarBg, dashItem, settItem, logItem)
	sidebarContainer := container.NewStack(sidebarBg, container.NewPadded(sidebarList))

	// Vertical separator (theme-aware)
	sepLight := color.NRGBA{R: 0xd1, G: 0xd1, B: 0xd6, A: 0x60}
	sepDark := color.NRGBA{R: 0x48, G: 0x48, B: 0x4a, A: 0x60}
	sep := newThemeAwareRect(sepLight, sepDark)
	sep.SetMinSize(fyne.NewSize(0.5, 1))
	themeDependentWidgets = append(themeDependentWidgets, sep)
	sidebarBlock := container.NewBorder(nil, nil, nil, sep, sidebarContainer)

	// ── Header ──
	headerIcon := canvas.NewImageFromResource(getAppIcon())
	headerIcon.SetMinSize(fyne.NewSize(28, 28))
	headerIcon.FillMode = canvas.ImageFillContain

	headerTitle := widget.NewRichTextWithText("Computer Control")
	headerTitle.Segments[0].(*widget.TextSegment).Style.TextStyle = fyne.TextStyle{Bold: true}

	versionLabel := widget.NewRichTextWithText("v" + clientVersion)
	versionLabel.Segments[0].(*widget.TextSegment).Style.ColorName = theme.ColorNameDisabled
	versionLabel.Segments[0].(*widget.TextSegment).Style.SizeName = theme.SizeNameCaptionText

	header := container.NewHBox(headerIcon, headerTitle, versionLabel)
	headerPadded := container.NewPadded(header)

	// Thin header separator (theme-aware)
	headerSepLight := color.NRGBA{R: 0xd1, G: 0xd1, B: 0xd6, A: 0x40}
	headerSepDark := color.NRGBA{R: 0x48, G: 0x48, B: 0x4a, A: 0x40}
	headerSep := newThemeAwareRect(headerSepLight, headerSepDark)
	headerSep.SetMinSize(fyne.NewSize(1, 0.5))
	themeDependentWidgets = append(themeDependentWidgets, headerSep)

	// ── Footer (Hide / Quit — hover: blue bg + white text, right-aligned) ──
	hideBtn := newHoverButton("Hide", func() {
		mainWindow.Hide()
	})
	exitBtn := newHoverButton("Quit", func() {
		fyneApp.Quit()
	})
	themeDependentWidgets = append(themeDependentWidgets, hideBtn, exitBtn)

	footer := container.NewHBox(layout.NewSpacer(), hideBtn, exitBtn)
	footerPadded := container.NewPadded(footer)

	// ── Main layout ──
	mainSplit := container.NewBorder(nil, nil, sidebarBlock, nil, container.NewPadded(contentArea))
	body := container.NewBorder(
		container.NewVBox(headerPadded, headerSep),
		footerPadded,
		nil, nil,
		mainSplit,
	)
	return body
}

// ──────────────────────────────────────────────
// Dashboard
// ──────────────────────────────────────────────

func buildDashboardContent() fyne.CanvasObject {
	// Section title
	title := widget.NewRichTextWithText("Device Information")
	title.Segments[0].(*widget.TextSegment).Style.TextStyle = fyne.TextStyle{Bold: true}

	// Status dot
	statusDot = newStatusDotWidget()
	onHeartbeatSending = func(sending bool) {
		fyne.Do(func() {
			if statusDot == nil {
				return
			}
			if sending {
				statusDot.SetState(StatusDotSending)
			} else {
				if appState.Connected {
					statusDot.SetState(StatusDotOnline)
				} else {
					statusDot.SetState(StatusDotError)
				}
			}
		})
	}

	// Editable hostname
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
	portValue := widget.NewLabel(fmt.Sprintf("%d", flagPort))
	portValue.TextStyle = fyne.TextStyle{Monospace: true}

	stayAwakeLabel = widget.NewLabel("")
	stayAwakeLabel.TextStyle = fyne.TextStyle{Bold: true}
	onStayAwakeStateChanged = func(active bool) {
		fyne.Do(func() {
			if stayAwakeLabel != nil {
				if active {
					stayAwakeLabel.SetText("☕ Active")
				} else {
					stayAwakeLabel.SetText("Off")
				}
				stayAwakeLabel.Refresh()
			}
		})
	}
	if isStayAwakeActive() {
		stayAwakeLabel.SetText("☕ Active")
	} else {
		stayAwakeLabel.SetText("Off")
	}

	statusRow := statusDot.Object()

	// CPU Temperature label (updated periodically)
	tempLabel := widget.NewLabel("—")
	tempLabel.TextStyle = fyne.TextStyle{Monospace: true}
	updateTemp := func() {
		t := getCPUTemperatureMillidegree()
		fyne.Do(func() {
			if t > 0 {
				tempLabel.SetText(fmt.Sprintf("🌡️ %d°C", t/1000))
			} else {
				tempLabel.SetText("N/A")
			}
		})
	}
	// Initial read + periodic refresh every 10s
	go func() {
		updateTemp()
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			updateTemp()
		}
	}()

	// ── Two-column info table (Label: grey, Value: black) ──
	infoGrid := container.New(
		layout.NewFormLayout(),
		// Row 1: Version
		secondaryLabel("Version"), widget.NewLabel(clientVersion),
		// Row 2: Computer Name
		secondaryLabel("Computer Name"), hostEntry,
		// Row 3: Local IP
		secondaryLabel("Local IP"), ipLabel,
		// Row 4: MAC Address
		secondaryLabel("MAC Address"), macValue,
		// Row 5: Operating System
		secondaryLabel("Operating System"), osValue,
		// Row 6: Listening Port
		secondaryLabel("Listening Port"), portValue,
		// Row 7: CPU Temperature
		secondaryLabel("CPU Temperature"), tempLabel,
		// Row 8: Status
		secondaryLabel("Status"), statusRow,
		// Row 9: Anti-Sleep
		secondaryLabel("Anti-Sleep"), stayAwakeLabel,
	)

	// ── Plugin URL section ──
	pluginLabel := widget.NewRichTextWithText("Homebridge Plugin URL")
	pluginLabel.Segments[0].(*widget.TextSegment).Style.TextStyle = fyne.TextStyle{Bold: true}

	pluginEntry := widget.NewEntry()
	pluginEntry.SetText(flagPluginURL)
	pluginEntry.SetPlaceHolder("http://homebridge-ip:9090")
	pluginEntry.OnChanged = func(val string) {
		flagPluginURL = val
		fyneApp.Preferences().SetString("plugin-url", val)
	}

	testBtn := widget.NewButton("Test", func() {
		statusDot.SetState(StatusDotSending)
		go func() {
			ok := sendHeartbeat(appState.Hostname, appState.IP, appState.MAC)
			updateConnectionStatus(ok)
		}()
	})
	testBtn.Importance = widget.HighImportance

	pluginRow := container.NewBorder(nil, nil, nil, testBtn, pluginEntry)

	// Thin separator (theme-aware)
	pluginSepLight := color.NRGBA{R: 0xd1, G: 0xd1, B: 0xd6, A: 0x40}
	pluginSepDark := color.NRGBA{R: 0x48, G: 0x48, B: 0x4a, A: 0x40}
	pluginSep := newThemeAwareRect(pluginSepLight, pluginSepDark)
	pluginSep.SetMinSize(fyne.NewSize(1, 0.5))
	themeDependentWidgets = append(themeDependentWidgets, pluginSep)

	return container.NewVBox(
		title,
		container.NewPadded(infoGrid),
		pluginSep,
		container.NewVBox(
			pluginLabel,
			pluginRow,
		),
		layout.NewSpacer(),
	)
}

// ──────────────────────────────────────────────
// Settings
// ──────────────────────────────────────────────

func buildSettingsContent() fyne.CanvasObject {
	title := widget.NewRichTextWithText("Settings")
	title.Segments[0].(*widget.TextSegment).Style.TextStyle = fyne.TextStyle{Bold: true}

	sendTempCheck := widget.NewCheck("Send Temperature Data", func(checked bool) {
		setSendTemperature(checked)
	})
	sendTempCheck.Checked = getSendTemperature()

	autoStartCheck := widget.NewCheck("Run at Startup (Auto-Start)", func(checked bool) {
		var asErr error
		if checked {
			asErr = enableAutoStart()
		} else {
			asErr = disableAutoStart()
		}
		if asErr != nil {
			dialog.ShowError(fmt.Errorf("Failed to set auto-start: %v", asErr), mainWindow)
		}
	})
	autoStartCheck.Checked = isAutoStartEnabled()

	return container.NewVBox(
		title,
		layout.NewSpacer(),
		container.NewPadded(container.NewVBox(
			sendTempCheck,
			autoStartCheck,
		)),
		layout.NewSpacer(),
	)
}

// ──────────────────────────────────────────────
// Logs
// ──────────────────────────────────────────────

func buildLogsContent() fyne.CanvasObject {
	title := widget.NewRichTextWithText("Log")
	title.Segments[0].(*widget.TextSegment).Style.TextStyle = fyne.TextStyle{Bold: true}

	logEntry := widget.NewMultiLineEntry()
	logEntry.SetMinRowsVisible(10)
	logEntry.Disable()
	logEntry.Wrapping = fyne.TextWrapWord
	logEntry.TextStyle = fyne.TextStyle{Monospace: true}

	logContainer := container.NewPadded(logEntry)

	updateLog := func() {
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

	refreshLogView = func() {
		fyne.Do(func() {
			if logEntry != nil {
				updateLog()
			}
		})
	}

	// Show any existing log entries immediately
	updateLog()

	clearLogBtn := widget.NewButton("Clear", func() {
		clearLog()
	})
	clearLogBtn.Importance = widget.LowImportance

	logToolbar := container.NewHBox(layout.NewSpacer(), clearLogBtn)

	return container.NewVBox(
		title,
		container.NewBorder(nil, logToolbar, nil, nil, logContainer),
	)
}
