package main

import (
	"fmt"
	"image/color"
	"strings"
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
	actionsContent := buildActionsContent()
	managedAppsContent := buildManagedAppsContent()
	settingsContent := buildSettingsContent()
	logsContent := buildLogsContent()

	// ── Sidebar items ──
	var dashItem, actItem, managedItem, settItem, logItem *sidebarItem
	selectItem := func(which *sidebarItem) {
		dashItem.SetSelected(which == dashItem)
		actItem.SetSelected(which == actItem)
		managedItem.SetSelected(which == managedItem)
		settItem.SetSelected(which == settItem)
		logItem.SetSelected(which == logItem)
	}

	dashItem = newSidebarItem("Dashboard", theme.HomeIcon(), func() {
		selectItem(dashItem)
		contentArea.Objects = []fyne.CanvasObject{dashboardContent}
		contentArea.Refresh()
	})
	actItem = newSidebarItem("Actions", theme.ContentAddIcon(), func() {
		selectItem(actItem)
		contentArea.Objects = []fyne.CanvasObject{actionsContent}
		contentArea.Refresh()
	})
	managedItem = newSidebarItem("Managed Apps", theme.ComputerIcon(), func() {
		selectItem(managedItem)
		contentArea.Objects = []fyne.CanvasObject{managedAppsContent}
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
		actItem,
		managedItem,
		settItem,
		logItem,
		layout.NewSpacer(),
	)

	// Sidebar background (theme-aware: light/dark)
	sidebarBg := newThemeAwareRect(macLightSidebar, macDarkSidebar)
	themeDependentWidgets = append(themeDependentWidgets, sidebarBg, dashItem, actItem, managedItem, settItem, logItem)
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
	lockPreventionLabel = widget.NewLabel("")
	lockPreventionLabel.TextStyle = fyne.TextStyle{Bold: true}
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
			if lockPreventionLabel != nil {
				if active {
					lockPreventionLabel.SetText("🔓 Active")
				} else {
					lockPreventionLabel.SetText("Off")
				}
				lockPreventionLabel.Refresh()
			}
		})
	}
	if isStayAwakeActive() {
		stayAwakeLabel.SetText("☕ Active")
		lockPreventionLabel.SetText("🔓 Active")
	} else {
		stayAwakeLabel.SetText("Off")
		lockPreventionLabel.SetText("Off")
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
		// Row 10: Lock Prevention
		secondaryLabel("Lock Prevention"), lockPreventionLabel,
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
			recordHeartbeatResult(ok)
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
// Actions
// ──────────────────────────────────────────────

func buildActionsContent() fyne.CanvasObject {
	title := widget.NewRichTextWithText("Actions")
	title.Segments[0].(*widget.TextSegment).Style.TextStyle = fyne.TextStyle{Bold: true}

	desc := widget.NewRichTextWithText("Define custom actions that appear as switches or buttons in HomeKit. Use {status} in Value to inject on/off.")
	desc.Segments[0].(*widget.TextSegment).Style.ColorName = theme.ColorNameDisabled
	desc.Wrapping = fyne.TextWrapWord

	actionsList := container.NewVBox()
	var refreshList func()
	refreshList = func() {
		actionsList.Objects = nil
		for _, a := range getActions() {
			aa := a
			interfaceLabel := a.Interface
			if l, ok := ActionInterfaceLabels[a.Interface]; ok {
				interfaceLabel = l
			}
			delBtn := widget.NewButton("Delete", func() {
				actions := getActions()
				var out []Action
				for _, x := range actions {
					if x.Name != aa.Name {
						out = append(out, x)
					}
				}
				setActions(out)
				refreshList()
			})
			delBtn.Importance = widget.HighImportance
			row := container.NewHBox(
				widget.NewLabel(a.Name),
				widget.NewLabel(a.Type),
				widget.NewLabel(interfaceLabel),
				layout.NewSpacer(),
				delBtn,
			)
			actionsList.Add(row)
		}
		actionsList.Refresh()
	}
	refreshList()

	nameEntry := widget.NewEntry()
	nameEntry.SetPlaceHolder("HomeKit display name")
	nameHint := widget.NewRichTextWithText("To avoid confusion, we recommend using the format \"Computer Name - Action Name\".")
	nameHint.Wrapping = fyne.TextWrapWord
	nameHint.Segments[0].(*widget.TextSegment).Style.ColorName = theme.ColorNameDisabled
	nameHint.Segments[0].(*widget.TextSegment).Style.SizeName = theme.SizeNameCaptionText
	typeSelect := widget.NewSelect(ActionTypesForPlatform(), nil)
	typeSelect.SetSelected(ActionTypeShell)
	valueEntry := widget.NewEntry()
	valueEntry.SetPlaceHolder("Command, script path, or URL. Use {status} for on/off")
	valueEntry.Wrapping = fyne.TextWrapWord
	valueEntry.MultiLine = true
	valueEntry.SetMinRowsVisible(2)
	bttHint := widget.NewRichTextWithText("")
	bttHint.Wrapping = fyne.TextWrapWord
	bttHint.Segments[0].(*widget.TextSegment).Style.ColorName = theme.ColorNameDisabled
	bttHint.Segments[0].(*widget.TextSegment).Style.SizeName = theme.SizeNameCaptionText
	urlModeSelect := widget.NewSelect([]string{
		URLModeLabels[URLModeFetch],
		URLModeLabels[URLModeBrowser],
	}, nil)
	urlModeSelect.SetSelected(URLModeLabels[URLModeFetch])
	urlModeRow := container.NewVBox(secondaryLabel("URL mode"), urlModeSelect)
	updateBTTHint := func() {
		if typeSelect.Selected == ActionTypeBTTTrigger {
			bttHint.Segments[0].(*widget.TextSegment).Text = "BTT: Full CLI command (bttcli auto-prefixed). Enable CLI in BTT Scripting Settings first. Examples: trigger_named \"mute\", execute_assigned_actions_for_trigger <UUID>, display_notification \"Hi\""
			valueEntry.SetPlaceHolder("trigger_named \"mute\" | execute_assigned_actions_for_trigger <UUID> | ...")
			urlModeRow.Hide()
		} else if typeSelect.Selected == ActionTypeAppleScript {
			bttHint.Segments[0].(*widget.TextSegment).Text = "File path (.applescript/.scpt) or inline script. Path: ~/path/to/script.applescript. Inline: display notification \"Hi\" with title \"Title\""
			valueEntry.SetPlaceHolder("~/Downloads/example.applescript | display notification \"Hi\"")
			urlModeRow.Hide()
		} else if typeSelect.Selected == ActionTypeURL {
			bttHint.Segments[0].(*widget.TextSegment).Text = ""
			valueEntry.SetPlaceHolder("URL (e.g. https://example.com/action). Use {status} for on/off")
			urlModeRow.Show()
		} else {
			bttHint.Segments[0].(*widget.TextSegment).Text = ""
			valueEntry.SetPlaceHolder("Command, script path, or URL. Use {status} for on/off")
			urlModeRow.Hide()
		}
		bttHint.Refresh()
		valueEntry.Refresh()
	}
	interfaceHint := widget.NewRichTextWithText("")
	interfaceHint.Wrapping = fyne.TextWrapWord
	interfaceHint.Segments[0].(*widget.TextSegment).Style.ColorName = theme.ColorNameDisabled
	interfaceHint.Segments[0].(*widget.TextSegment).Style.SizeName = theme.SizeNameCaptionText
	interfaceSelect := widget.NewSelect([]string{
		ActionInterfaceLabels[ActionInterfaceToggle],
		ActionInterfaceLabels[ActionInterfaceButton],
	}, nil)
	interfaceSelect.SetSelected(ActionInterfaceLabels[ActionInterfaceToggle])
	wakeBeforeRadio := widget.NewRadioGroup([]string{"Yes", "No"}, nil)
	wakeBeforeRadio.SetSelected("No")
	wakeBeforeHint := widget.NewRichTextWithText("Same as standard wake: WoL → 5s delay → wake-screen (display on) → run-action.")
	wakeBeforeHint.Wrapping = fyne.TextWrapWord
	wakeBeforeHint.Segments[0].(*widget.TextSegment).Style.ColorName = theme.ColorNameDisabled
	wakeBeforeHint.Segments[0].(*widget.TextSegment).Style.SizeName = theme.SizeNameCaptionText
	sleepAfterRadio := widget.NewRadioGroup([]string{"Yes", "No"}, nil)
	sleepAfterRadio.SetSelected("No")
	sleepAfterHint := widget.NewRichTextWithText("5s after action triggers, client runs OS sleep (pmset/rundll32/systemctl).")
	sleepAfterHint.Wrapping = fyne.TextWrapWord
	sleepAfterHint.Segments[0].(*widget.TextSegment).Style.ColorName = theme.ColorNameDisabled
	sleepAfterHint.Segments[0].(*widget.TextSegment).Style.SizeName = theme.SizeNameCaptionText
	updateInterfaceHint := func() {
		if interfaceSelect.Selected == ActionInterfaceLabels[ActionInterfaceButton] {
			interfaceHint.Segments[0].(*widget.TextSegment).Text = "Push Button: {status} is always \"on\" — no Off concept. Use for one-shot actions."
		} else {
			interfaceHint.Segments[0].(*widget.TextSegment).Text = ""
		}
		interfaceHint.Refresh()
	}
	typeSelect.OnChanged = func(_ string) { updateBTTHint(); updateInterfaceHint() }
	interfaceSelect.OnChanged = func(_ string) { updateInterfaceHint() }
	urlModeRow.Hide()

	addBtn := widget.NewButton("Add Action", func() {
		name := strings.TrimSpace(nameEntry.Text)
		if name == "" {
			dialog.ShowError(fmt.Errorf("Name is required"), mainWindow)
			return
		}
		actions := getActions()
		for _, x := range actions {
			if x.Name == name {
				dialog.ShowError(fmt.Errorf("Action with name %q already exists", name), mainWindow)
				return
			}
		}
		interfaceVal := ActionInterfaceToggle
		if interfaceSelect.Selected == ActionInterfaceLabels[ActionInterfaceButton] {
			interfaceVal = ActionInterfaceButton
		}
		a := Action{
			Name:             name,
			Type:             NormalizeActionType(typeSelect.Selected),
			Value:            strings.TrimSpace(valueEntry.Text),
			Interface:        interfaceVal,
			WakeBeforeAction: wakeBeforeRadio.Selected == "Yes",
			SleepAfterAction: sleepAfterRadio.Selected == "Yes",
		}
		if a.Type == ActionTypeURL {
			if urlModeSelect.Selected == URLModeLabels[URLModeBrowser] {
				a.URLMode = URLModeBrowser
			} else {
				a.URLMode = URLModeFetch
			}
		}
		if a.Value == "" {
			dialog.ShowError(fmt.Errorf("Value is required"), mainWindow)
			return
		}
		setActions(append(actions, a))
		nameEntry.SetText("")
		valueEntry.SetText("")
		refreshList()
	})
	addBtn.Importance = widget.HighImportance

	updateBTTHint()
	updateInterfaceHint()

	form := container.NewVBox(
		secondaryLabel("Name"),
		nameEntry,
		nameHint,
		secondaryLabel("Type"),
		typeSelect,
		secondaryLabel("Value (command, path, or URL)"),
		valueEntry,
		bttHint,
		urlModeRow,
		secondaryLabel("Interface"),
		interfaceSelect,
		interfaceHint,
		secondaryLabel("Wake Computer Before Action"),
		wakeBeforeRadio,
		wakeBeforeHint,
		secondaryLabel("Sleep Device After Action"),
		sleepAfterRadio,
		sleepAfterHint,
		addBtn,
	)

	sepLight := color.NRGBA{R: 0xd1, G: 0xd1, B: 0xd6, A: 0x40}
	sepDark := color.NRGBA{R: 0x48, G: 0x48, B: 0x4a, A: 0x40}
	sep := newThemeAwareRect(sepLight, sepDark)
	sep.SetMinSize(fyne.NewSize(1, 0.5))
	themeDependentWidgets = append(themeDependentWidgets, sep)

	// Entire tab scrolls as one; no inner scroll on actions list; bottom padding
	content := container.NewVBox(
		title,
		desc,
		container.NewPadded(form),
		sep,
		widget.NewRichTextWithText("Current Actions"),
		actionsList,
		layout.NewSpacer(),
		layout.NewSpacer(), // extra bottom padding
	)
	return container.NewScroll(container.NewPadded(content))
}

// ──────────────────────────────────────────────
// Managed Apps
// ──────────────────────────────────────────────

func buildManagedAppsContent() fyne.CanvasObject {
	title := widget.NewRichTextWithText("Managed Apps")
	title.Segments[0].(*widget.TextSegment).Style.TextStyle = fyne.TextStyle{Bold: true}

	desc := widget.NewRichTextWithText("Monitor and control app state in HomeKit. ON = app running, OFF = app not running. Add apps to create switches in Home.")
	desc.Segments[0].(*widget.TextSegment).Style.ColorName = theme.ColorNameDisabled
	desc.Wrapping = fyne.TextWrapWord

	managedAppsList := container.NewVBox()
	var refreshList func()
	refreshList = func() {
		managedAppsList.Objects = nil
		for _, app := range getManagedApps() {
			aa := app
			delBtn := widget.NewButton("Delete", func() {
				apps := getManagedApps()
				var out []ManagedAppEntry
				for _, x := range apps {
					if !strings.EqualFold(x.Name, aa.Name) {
						out = append(out, x)
					}
				}
				setManagedApps(out)
				refreshList()
			})
			delBtn.Importance = widget.HighImportance
			badges := ""
			switch aa.QuitMode {
			case QuitModeQuit:
				badges += " [Graceful]"
			case QuitModeQuitKill:
				badges += " [Smart]"
			default:
				badges += " [Force]"
			}
			if aa.WakeBefore {
				badges += " [Wake]"
			}
			if aa.SleepAfter {
				badges += " [Sleep]"
			}
			label := app.Name
			if app.DisplayName != "" {
				label = app.DisplayName + " (" + app.Name + ")"
			}
			row := container.NewHBox(
				widget.NewLabel(label+badges),
				layout.NewSpacer(),
				delBtn,
			)
			managedAppsList.Add(row)
		}
		managedAppsList.Refresh()
	}
	refreshList()

	addBtn := widget.NewButton("Add", func() {
		// Get running process names when dialog opens (instant refresh)
		allOptions := GetRunningProcessNames()
		filtered := make([]string, len(allOptions))
		copy(filtered, allOptions)

		processEntry := widget.NewEntry()
		processEntry.SetPlaceHolder("Type to search or select from list below")

		processList := widget.NewList(
			func() int { return len(filtered) },
			func() fyne.CanvasObject { return widget.NewLabel("") },
			func(id widget.ListItemID, o fyne.CanvasObject) {
				o.(*widget.Label).SetText(filtered[id])
			},
		)
		processList.OnSelected = func(id widget.ListItemID) {
			if id >= 0 && id < len(filtered) {
				processEntry.SetText(filtered[id])
			}
		}

		updateFilter := func() {
			q := strings.TrimSpace(strings.ToLower(processEntry.Text))
			filtered = filtered[:0]
			for _, s := range allOptions {
				if q == "" || strings.Contains(strings.ToLower(s), q) {
					filtered = append(filtered, s)
				}
			}
			processList.Refresh()
		}
		processEntry.OnChanged = func(_ string) { updateFilter() }
		updateFilter()

		listScroll := container.NewScroll(processList)
		listScroll.SetMinSize(fyne.NewSize(360, 180))

		displayNameEntry := widget.NewEntry()
		displayNameEntry.SetPlaceHolder("e.g. Firefox (optional, shown in HomeKit)")

		wakeBeforeRadio := widget.NewRadioGroup([]string{"Yes", "No"}, nil)
		wakeBeforeRadio.SetSelected("No")
		wakeBeforeHint := widget.NewRichTextWithText("Same as standard wake: WoL → 5s delay → wake-screen (display on) → launch app.")
		wakeBeforeHint.Wrapping = fyne.TextWrapWord
		wakeBeforeHint.Segments[0].(*widget.TextSegment).Style.ColorName = theme.ColorNameDisabled
		wakeBeforeHint.Segments[0].(*widget.TextSegment).Style.SizeName = theme.SizeNameCaptionText
		sleepAfterRadio := widget.NewRadioGroup([]string{"Yes", "No"}, nil)
		sleepAfterRadio.SetSelected("No")
		sleepAfterHint := widget.NewRichTextWithText("5s after app is quit, client runs OS sleep (pmset/rundll32/systemctl).")
		sleepAfterHint.Wrapping = fyne.TextWrapWord
		sleepAfterHint.Segments[0].(*widget.TextSegment).Style.ColorName = theme.ColorNameDisabled
		sleepAfterHint.Segments[0].(*widget.TextSegment).Style.SizeName = theme.SizeNameCaptionText
		quitModeSelect := widget.NewSelect([]string{
			"Standard Quit (Graceful)",
			"Force Quit (Immediate)",
			"Smart Quit (Wait then Kill)",
		}, nil)
		quitModeSelect.SetSelected("Standard Quit (Graceful)")
		quitModeHint := widget.NewRichTextWithText("Standard Quit: graceful (osascript/SIGTERM). Force Quit: immediate terminate. Smart Quit: try graceful, wait 4s, then force if still running.")
		quitModeHint.Wrapping = fyne.TextWrapWord
		quitModeHint.Segments[0].(*widget.TextSegment).Style.ColorName = theme.ColorNameDisabled
		quitModeHint.Segments[0].(*widget.TextSegment).Style.SizeName = theme.SizeNameCaptionText
		modalContent := container.NewVBox(
			widget.NewLabel("Process name (type to search, click to select):"),
			processEntry,
			widget.NewLabel("Running processes:"),
			listScroll,
			widget.NewLabel("HomeKit display name (optional):"),
			displayNameEntry,
			widget.NewLabel("When turning OFF:"),
			quitModeSelect,
			quitModeHint,
			widget.NewLabel("Wake Computer Before Launch"),
			wakeBeforeRadio,
			wakeBeforeHint,
			widget.NewLabel("Sleep Device After Quit"),
			sleepAfterRadio,
			sleepAfterHint,
		)
		modalContent.Resize(fyne.NewSize(420, 580))

		dlg := dialog.NewCustomConfirm("Add Managed App", "Add", "Cancel", modalContent, func(ok bool) {
			if !ok {
				return
			}
			name := strings.TrimSpace(processEntry.Text)
			if name == "" {
				dialog.ShowError(fmt.Errorf("App name is required"), mainWindow)
				return
			}
			apps := getManagedApps()
			for _, x := range apps {
				if strings.EqualFold(x.Name, name) {
					dialog.ShowError(fmt.Errorf("App %q is already managed", name), mainWindow)
					return
				}
			}
			quitMode := QuitModeKill
			switch quitModeSelect.Selected {
			case "Standard Quit (Graceful)":
				quitMode = QuitModeQuit
			case "Smart Quit (Wait then Kill)":
				quitMode = QuitModeQuitKill
			case "Force Quit (Immediate)":
			default:
				quitMode = QuitModeKill
			}
			entry := ManagedAppEntry{
				Name:        name,
				DisplayName: strings.TrimSpace(displayNameEntry.Text),
				WakeBefore:  wakeBeforeRadio.Selected == "Yes",
				SleepAfter:  sleepAfterRadio.Selected == "Yes",
				QuitMode:    quitMode,
			}
			setManagedApps(append(apps, entry))
			refreshList()
		}, mainWindow)
		dlg.Resize(fyne.NewSize(460, 620))
		dlg.Show()
		// Focus entry when dialog opens so user can type immediately
		go func() {
			time.Sleep(80 * time.Millisecond)
			if c := mainWindow.Canvas(); c != nil {
				c.Focus(processEntry)
			}
		}()
	})
	addBtn.Importance = widget.HighImportance

	sepLight := color.NRGBA{R: 0xd1, G: 0xd1, B: 0xd6, A: 0x40}
	sepDark := color.NRGBA{R: 0x48, G: 0x48, B: 0x4a, A: 0x40}
	sep := newThemeAwareRect(sepLight, sepDark)
	sep.SetMinSize(fyne.NewSize(1, 0.5))
	themeDependentWidgets = append(themeDependentWidgets, sep)

	content := container.NewVBox(
		title,
		desc,
		addBtn,
		sep,
		widget.NewRichTextWithText("Current Managed Apps"),
		managedAppsList,
		layout.NewSpacer(),
	)
	return container.NewScroll(container.NewPadded(content))
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

	screensaverCheck := widget.NewCheck("Enable Remote Screensaver", func(checked bool) {
		setEnableRemoteScreensaver(checked)
	})
	screensaverCheck.Checked = getEnableRemoteScreensaver()

	lockCheck := widget.NewCheck("Enable Remote Lock", func(checked bool) {
		setEnableRemoteLock(checked)
	})
	lockCheck.Checked = getEnableRemoteLock()

	var joinAntiSleepCheck *widget.Check
	enableAntiSleepCheck := widget.NewCheck("Enable Anti-Sleep (individual)", func(checked bool) {
		setEnableAntiSleep(checked)
		if checked && joinAntiSleepCheck != nil {
			joinAntiSleepCheck.Checked = false
			joinAntiSleepCheck.Refresh()
		}
	})
	enableAntiSleepCheck.Checked = getEnableAntiSleep()

	joinAntiSleepCheck = widget.NewCheck("Join Anti-Sleep (global)", func(checked bool) {
		setJoinAntiSleep(checked)
		if checked {
			enableAntiSleepCheck.Checked = false
			enableAntiSleepCheck.Refresh()
		}
	})
	joinAntiSleepCheck.Checked = getJoinAntiSleep()

	var joinLockPrevCheck *widget.Check
	enableLockPrevCheck := widget.NewCheck("Enable Lock Prevention (individual)", func(checked bool) {
		setEnableLockPrevention(checked)
		if checked && joinLockPrevCheck != nil {
			joinLockPrevCheck.Checked = false
			joinLockPrevCheck.Refresh()
		}
	})
	enableLockPrevCheck.Checked = getEnableLockPrevention()

	joinLockPrevCheck = widget.NewCheck("Join Lock Prevention (global)", func(checked bool) {
		setJoinLockPrevention(checked)
		if checked {
			enableLockPrevCheck.Checked = false
			enableLockPrevCheck.Refresh()
		}
	})
	joinLockPrevCheck.Checked = getJoinLockPrevention()

	var joinMasterCheck *widget.Check
	volumeSliderCheck := widget.NewCheck("Enable Volume Slider", func(checked bool) {
		setEnableVolumeSlider(checked)
		if checked && joinMasterCheck != nil {
			joinMasterCheck.Checked = false
			joinMasterCheck.Refresh()
		}
	})
	volumeSliderCheck.Checked = getEnableVolumeSlider()

	joinMasterCheck = widget.NewCheck("Join Master Volume", func(checked bool) {
		setJoinMasterVolume(checked)
		if checked {
			volumeSliderCheck.Checked = false
			volumeSliderCheck.Refresh()
		}
	})
	joinMasterCheck.Checked = getJoinMasterVolume()

	volumeNameEntry := widget.NewEntry()
	volumeNameEntry.SetPlaceHolder("e.g. MacBook - Volume (default)")
	volumeNameEntry.SetText(getVolumeSliderName())
	volumeNameEntry.OnChanged = func(s string) {
		setVolumeSliderName(strings.TrimSpace(s))
	}
	volumeNameRow := container.NewBorder(nil, nil, widget.NewLabel("Volume Slider Name:"), nil, volumeNameEntry)

	heartbeatEntry := widget.NewEntry()
	heartbeatEntry.SetPlaceHolder("5-300, default 30")
	heartbeatEntry.SetText(fmt.Sprintf("%d", getHeartbeatIntervalSec()))
	heartbeatEntry.OnChanged = func(s string) {
		var v int
		if _, err := fmt.Sscanf(s, "%d", &v); err == nil && v >= 5 && v <= 300 {
			setHeartbeatIntervalSec(v)
		}
	}
	heartbeatRow := container.NewBorder(nil, nil, widget.NewLabel("Heartbeat Interval (sec):"), nil, heartbeatEntry)

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
			screensaverCheck,
			lockCheck,
			enableAntiSleepCheck,
			joinAntiSleepCheck,
			enableLockPrevCheck,
			joinLockPrevCheck,
			volumeSliderCheck,
			joinMasterCheck,
			volumeNameRow,
			heartbeatRow,
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
