# 🏠 HomeBridge Computer Control

[![npm version](https://img.shields.io/npm/v/homebridge-computer-control.svg)](https://www.npmjs.com/package/homebridge-computer-control)
[![npm downloads](https://img.shields.io/npm/dm/homebridge-computer-control.svg)](https://www.npmjs.com/package/homebridge-computer-control)
[![License: MIT](https://img.shields.io/github/license/orkank/homebridge-computer-control.svg)](https://opensource.org/licenses/MIT)
[![GitHub stars](https://img.shields.io/github/stars/orkank/homebridge-computer-control.svg)](https://github.com/orkank/homebridge-computer-control/stargazers)
[![GitHub forks](https://img.shields.io/github/forks/orkank/homebridge-computer-control.svg)](https://github.com/orkank/homebridge-computer-control/network)

<p align="center">
  <img src="homekit1.png" alt="Home app" width="400">
  <img src="homekit2.png" alt="Client app" width="400">
</p>

<p align="center">
  <img src="client1.png" alt="Home app" width="400">
  <img src="client2.png" alt="Client app" width="400">
  <img src="client3.png" alt="Client app" width="400">
  <img src="client4.png" alt="Client app" width="400">
</p>

> ✅ **Ready for use** - This plugin is production-ready.

Control your computers (macOS, Windows, Linux) through Apple HomeKit using Homebridge. Wake them with WoL, put them to sleep remotely, and manage them as HomeKit switches.

**Version:** 1.1.9

### 📱 Managed Apps - Live Application Monitoring (Real-State) *(1.1.6)*

**Managed Apps** lets you monitor and control running applications directly from HomeKit. Each app becomes a switch: **ON** = app is running, **OFF** = app is not running. Unlike fire-and-forget actions, the switch state reflects the **actual process state** from the client.

| Feature | Description |
|---------|-------------|
| **Real-State Switch** | Switch reflects only client-reported `app_states` - never from user tap. If you turn OFF in HomeKit, the switch stays ON until the app actually quits |
| **Add Process** | Client **Managed Apps** tab → Add → select from running processes (searchable list). macOS: clean `.app` names; Windows: includes `.exe` |
| **Launch / Quit** | Client `/manage-app?name=X&target=on\|off` - launch app or quit/kill based on **When turning OFF** setting |
| **When turning OFF** | **Standard Quit (Graceful)**, **Force Quit (Immediate)**, **Smart Quit (Wait then Kill)** |
| **Wake Before Launch** | If computer is already reachable, skip WoL; else WoL → 5s delay → wake-screen → launch app |
| **Sleep After Quit** | Same as Actions: 5 seconds after quit, client runs OS sleep (pmset/rundll32/systemctl) |

Storage: `managed_apps.json` in the same config dir as `actions.json`. Each app: `{name, wakeBefore, sleepAfter, quitMode}`.

### 🔊 Volume Control

Control system volume from HomeKit using Lightbulb/Brightness (slider). Two modes - **mutually exclusive**:

| Mode | Description |
|------|-------------|
| **Individual Slider** | Client Settings → Enable Volume Slider. Each device gets its own volume accessory (e.g. "MacBook - Volume"). |
| **Master Volume** | Client Settings → Join Master Volume. All devices in the group share one "Computer Volume" slider. When one device changes volume locally, all others sync automatically. |

| Step | Description |
|------|-------------|
| **Client** | Settings → Enable Volume Slider (individual) **or** Join Master Volume (group). One disables the other. |
| **Client** | Volume Slider Name: custom name for individual slider (default: `[Hostname] - Volume`) |
| **Plugin** | Config → Enable Global Volume (Master Slider) + Master Volume Accessory Name (default: "Computer Volume") |
| **Real-time sync** | When Join Master Volume: client polls volume every 2s; if changed locally (keyboard), notifies plugin → plugin broadcasts to all other devices in the group |

macOS: `osascript` output volume. Windows: PowerShell + COM (IAudioEndpointVolume). Linux: `pactl` (PulseAudio) or `amixer` (ALSA).

### 🖼️ Screensaver Sync

> ⚠️ **Experimental** - May not work stably on all setups.

Start the screensaver on all computers from a single HomeKit switch.

| Step | Description |
|------|-------------|
| **Client** | Settings → Enable Remote Screensaver |
| **Plugin** | Config → Enable Global Screensaver Switch |
| **HomeKit** | "All Screensavers" switch appears; tap ON → screensaver starts on all enabled **and online** clients |
| **Online only** | Only reachable clients receive the command (2s health check); sleeping/offline devices are skipped |
| **Feedback** | Switch stays ON ~1.5s then auto-resets to OFF (push button) |

macOS: `ScreenSaverEngine`; Windows: `scrnsave.scr /s`; Linux: `xdg-screensaver activate` (fallback: xscreensaver-command).

### 🔒 Lock Computers

> ⚠️ **Experimental** - May not work stably on all setups.

Lock the screen on all computers from a single HomeKit switch. **Does not put computers to sleep** - only locks the display.

| Step | Description |
|------|-------------|
| **Client** | Settings → Enable Remote Lock |
| **Plugin** | Config → Enable Global Lock Switch |
| **HomeKit** | "Lock Computers" switch appears; tap ON → lock screen on all online enabled clients |
| **Online only** | Same as Screensaver - only reachable clients receive the command |

macOS: osascript (Lock shortcut or Finder menu; Accessibility may be required), then `pmset displaysleepnow` fallback. Windows: `rundll32 user32.dll,LockWorkStation`. Linux: `xdg-screensaver lock` (fallback: xscreensaver-command -lock).

### 🎯 Custom Actions - One-Tap HomeKit Accessories

Define actions in the client **Actions** tab; they appear instantly as switches or buttons in HomeKit. No manual config - every action you add shows up automatically in the Home app.

| Platform | Available Actions |
|----------|------------------------|
| **macOS** | **BTT** (BetterTouchTool), Shell, Batch, AppleScript, URL |
| **Linux** | Shell, Batch, URL |
| **Windows** | Batch, URL |

**macOS + BetterTouchTool:** With BTT installed, you can bind any BTT CLI command to a HomeKit accessory with one tap. Commands like `trigger_named "mute"`, `display_notification "Hello"`, or `set_string_variable` - add them in the client Actions tab → they appear as switches/buttons in Home → trigger via Siri or automations.

> **Note:** BTT's Command Line / Socket Server must be enabled in BetterTouchTool's Scripting Settings first. See [BTT CLI documentation](https://docs.folivora.ai/docs/scripting/cli) for details.

**Trigger by UUID:** You can also trigger any BTT trigger by its UUID: `execute_assigned_actions_for_trigger 823A845F-8D62-4950-8709-1CE5527CEADF`. Find the UUID in BTT (right-click trigger → Copy UUID) and add it as an action - no need to use the trigger name.

**Example uses:** Mute/unmute, show notifications, set variables, open URLs, run scripts, trigger shortcuts. Choose Toggle (remembers state) or Push Button (one-shot).

**Wake Before / Sleep After:** Each action can optionally enable:
- **Wake Computer Before Action** - Same flow as standard wake: WoL → 5s delay → wake-screen (display on) → run-action. Ensures display wakes from Dark Wake.
- **Sleep Device After Action** - 5 seconds after the action triggers, the client runs the OS sleep command (macOS: pmset sleepnow, Windows: rundll32, Linux: systemctl suspend).

## Features

| Feature | Description |
|---------|-------------|
| **Wake-on-LAN** | Wake sleeping computers from anywhere via HomeKit |
| **Remote Sleep** | Put computers to sleep with a single tap |
| **Group Control** | Virtual "Computers" accessory - Wake All / Sleep All in one command |
| **Auto-Registration** | Clients register automatically; no manual config needed |
| **Update Notification** | Clients receive a one-time update notification with download link |
| **macOS Power Nap** | Correctly detects Dark Wake; device stays OFF when display is asleep |
| **Token Auth** | Client and plugin use shared tokens; no unauthorized sleep/wake |
| **Config UI** | View clients, remove stale ones, configure group name |
| **Anti-Sleep** | Virtual switch to prevent computers from sleeping. Client: Join Anti-Sleep (global) or Enable Anti-Sleep (individual). Plugin: Enable Global Anti-Sleep. Optional timer. Same rules as Volume (mutual exclusion) |
| **Lock Prevention** | Virtual switch to prevent screen dimming and lock screen. Client: Join Lock Prevention (global) or Enable Lock Prevention (individual). Plugin: Enable Global Lock Prevention. Uses same stay-awake mechanism as Anti-Sleep |
| **Temperature Sensor** | Optional CPU temperature in HomeKit (client checkbox "Send Temperature Data"); Linux thermal/sensors, macOS ioreg, Windows WMI |
| **Custom Actions** | Define BTT, shell, batch, AppleScript, or URL actions in the client; they appear as switches or buttons in HomeKit |
| **Managed Apps** | Live app monitoring - add process names in client; each becomes a real-state switch (ON = running, OFF = not running). Standard/Force/Smart Quit; optional Wake Before Launch / Sleep After Quit; skip WoL when computer already online |
| **Screensaver Sync** | Client "Enable Remote Screensaver" + plugin "Enable Global Screensaver Switch" → "All Screensavers" push button in HomeKit; sends screensaver to all online enabled clients |
| **Lock Computers** | Client "Enable Remote Lock" + plugin "Enable Global Lock Switch" → "Lock Computers" push button; locks screen on all online enabled clients (does NOT put to sleep) |
| **Volume Control** | Individual slider per device or Master Volume (group). Lightbulb/Brightness = volume 0–100. Real-time sync when volume changed locally on any device in master group |

## Architecture

```
┌──────────────────────────────────────────────────────────┐
│                    Apple Home / HomeKit                  │
└────────────────────────────┬─────────────────────────────┘
                             │
┌────────────────────────────▼─────────────────────────────┐
│                   Homebridge Plugin                      │
│  ┌─────────────┐  ┌──────────────┐  ┌─────────────────┐  │
│  │ Registration│  │ Wake-on-LAN  │  │  Status Check   │  │
│  │   Server    │  │   (Power On) │  │  (HTTP)         │  │
│  └─────────────┘  └──────────────┘  └─────────────────┘  │
│  ┌──────────────────────────────────────────────────────┐│
│  │        📥 Binary Download Server (:9090)             ││
│  └──────────────────────────────────────────────────────┘│
└────────────────────────────┬─────────────────────────────┘
                             │ HTTP / WoL
        ┌────────────────────┼────────────────────┐
        │                    │                    │
┌───────▼──────┐  ┌──────────▼─────┐  ┌───────────▼────┐
│  Go Client   │  │   Go Client    │  │   Go Client    │
│  (macOS)     │  │   (Windows)    │  │   (Linux)      │
│  .app bundle │  │   .exe (GUI)   │  │   binary       │
│  Hidden Agent│  │   No Console   │  │   GUI          │
└──────────────┘  └────────────────┘  └────────────────┘
```

## Project Structure (Single NPM Package)

```
homebridge-computer-control/
├── package.json              ← Homebridge plugin manifest
├── tsconfig.json             ← TypeScript config
├── config.schema.json        ← Config UI X schema
├── .gitignore
├── README.md
│
├── src/                      ← Plugin TypeScript source
│   ├── index.ts              ← Plugin entry point
│   ├── platform.ts           ← Main platform (registration, WoL, downloads)
│   ├── platformAccessory.ts  ← HomeKit Switch handler
│   ├── settings.ts           ← Constants & interfaces
│   └── types.d.ts            ← Type declarations
│
├── dist/                     ← Compiled plugin (generated)
│
├── client/                   ← Go client source
│   ├── main.go               ← Client binary source
│   └── go.mod                ← Go module
│
├── examples/                 ← Example scripts for actions
│   ├── example.sh            ← Shell (macOS/Linux)
│   ├── example.bat           ← Batch (Windows)
│   └── example.applescript   ← AppleScript (macOS)
│
├── bin/                      ← Pre-compiled client binaries (generated)
│   ├── ComputerControl.app/  ← macOS hidden agent bundle
│   │   └── Contents/
│   │       ├── Info.plist     ← LSUIElement=true (no Dock icon)
│   │       └── MacOS/
│   │           └── client
│   ├── computer-control-darwin-app.zip
│   ├── computer-control-windows-amd64.exe
│   └── computer-control-linux-amd64[.tar.xz]  ← if built
│
└── scripts/
    └── build-clients.sh      ← Cross-compilation script
```

## Quick Start

### 1. Build Everything
```bash
# Install dependencies & build clients + plugin
npm install
npm run build:all
```

### 2. Build Only Clients (Go Binaries)
```bash
npm run build:clients
# or directly:
bash scripts/build-clients.sh
```

### 3. Install Plugin in Homebridge
```bash
npm run build
npm link
# or install globally for Homebridge
sudo npm install -g ./
```

### 4. Homebridge Config

> ⚠️ **Required** - The plugin will **not work** until the platform is added to your config. After installing, open the plugin settings in Homebridge Config UI X and **Save** the configuration. Or add the platform manually to `config.json`.

Add to your `config.json` (or use Config UI X → Plugins → Computer Control → Save):
```json
{
  "platforms": [
    {
      "platform": "ComputerControl",
      "name": "Computer Control",
      "registrationPort": 9090,
      "groupAccessoryName": "Computers",
      "antiSleepDeviceName": "Computer Sleep Prevention",
      "antiSleepTimer": 0,
      "enableGlobalScreensaverSwitch": false,
      "enableGlobalLockSwitch": false,
      "enableGlobalVolumeSwitch": false,
      "masterVolumeName": "Computer Volume",
      "clients": []
    }
  ]
}
```

**Docker:** If Homebridge runs in Docker, ensure port **9090** is exposed (registration + download server). Example: `-p 9090:9090`

### 5. Download & Run Client on Target Computers

Once the plugin is running, clients can be downloaded from:
```
http://<homebridge-ip>:9090/download
```

| Platform | Endpoint | Notes |
|---|---|---|
| macOS (.app bundle) | `/download/darwin-app` | Zip with .app; hidden agent (no Dock icon); see [macOS Gatekeeper](#macos-gatekeeper) below |
| Windows (64-bit) | `/download/windows-amd64` | No console window |
| Windows (ARM) | `/download/windows-arm64` | No console window |
| Linux (64-bit) | `/download/linux-amd64` | Standalone binary |
| Linux (ARM64) | `/download/linux-arm64` | For Raspberry Pi etc. |

#### macOS Gatekeeper

macOS quarantines apps downloaded from the internet. If you see **"ComputerControl.app is damaged and can't be opened"**, run this in Terminal before first launch:

```bash
xattr -cr ComputerControl.app
```

Alternatively: right-click the app → **Open** (first time only).

### 6. Run the Client
```bash
# macOS: Extract the zip, then run (see macOS Gatekeeper above for first launch)
open ComputerControl.app --args --plugin-url http://<homebridge-ip>:9090

# Windows: Run the .exe with plugin URL
computer-control-windows-amd64.exe --plugin-url http://<homebridge-ip>:9090

# Linux: Run the binary
./computer-control-linux-amd64 --plugin-url http://<homebridge-ip>:9090

# The client auto-detects IP, MAC, and OS (default port: 45991)
# Check version (CLI or GUI)
./computer-control-linux-amd64 --version   # or use GUI About
```

## Changelog

### 1.1.9 (Current)

- **macOS Dark Wake / health checks**: Fixed repeated `pmset -g log` subprocesses on Apple Silicon
  - Many Macs no longer expose "Display Asleep" lines in `system_profiler`; the old fallback spawned `pmset` on every `/health` poll, heartbeat, and volume poll
  - Display probe no longer uses `pmset -g log`; results are cached briefly (~3s) to reduce duplicate work
  - When display state is unknown, the client assumes full wake (not dark wake) instead of shelling out to `pmset`
- **macOS Remote Lock**: More reliable lock screen from HomeKit
  - Tries osascript (default Lock shortcut; may require **Accessibility** for Computer Control), then Finder **Lock Screen** menu, then `pmset displaysleepnow` fallback
  - Request logging in the client UI
- **macOS Remote Screensaver**: Multiple launch methods for newer macOS (Sonoma+)
  - `open -a ScreenSaverEngine`, full `.app` path, bundle ID `com.apple.ScreenSaver.Engine`, and direct binary as fallbacks
- **Client settings**: Configurable **heartbeat interval** (5–300 seconds, default 30) in Settings
- **Client**: Immediate heartbeat after toggling temperature, remote screensaver, remote lock, volume, anti-sleep, and lock-prevention preferences (faster HomeKit sync)
- **Managed Apps**: Optional **display name** per app for the HomeKit accessory title (process name remains the control key)
- **Plugin**: After client registration, **stay-awake state is synced** across clients so global and per-device Anti-Sleep / Lock Prevention stay consistent when the roster changes
- **Windows Anti-Sleep**: Improved reliability on Windows 11
  - Added `ES_DISPLAY_REQUIRED` (prevents both system and display sleep)
  - Periodic refresh every 60 seconds (Windows 11 can clear `SetThreadExecutionState`; refresh maintains the state)
  - Aligns with PowerToys Awake approach
- **Anti-Sleep Auto-Off**: Safety timeout when client cannot reach server
  - If client has no successful connection to server for 10 minutes, client turns off anti-sleep locally
  - Prevents devices from staying awake indefinitely when server/network is down
  - Applies only when plugin URL is configured and heartbeats are being sent
- **Lock Prevention**: New feature to prevent screen dimming and lock screen
  - Global virtual accessory (only shown when clients with Join Lock Prevention exist)
  - Client Settings: "Enable Lock Prevention (individual)" or "Join Lock Prevention (global)" — mutual exclusion like Volume
  - Plugin config: "Enable Global Lock Prevention" + Lock Prevention Device Name
  - Uses same stay-awake mechanism as Anti-Sleep
  - Client dashboard: Lock Prevention status indicator (Active/Off)
- **Anti-Sleep**: Refactored to Join/Individual pattern (same as Volume)
  - Client Settings: "Enable Anti-Sleep (individual)" or "Join Anti-Sleep (global)" — mutual exclusion
  - Plugin config: "Enable Global Anti-Sleep" — accessory only shown when participants exist
  - Per-device Anti-Sleep switch when individual mode enabled
- **Windows Screensaver**: Use user-configured screensaver from registry
  - Reads `HKCU\Control Panel\Desktop\SCRNSAVE.EXE` for user's selected screensaver
  - Falls back to `scrnsave.scr` if none configured or path invalid
  - Fixes issue where only screen dimmed instead of actual screensaver (e.g. Photos, Bubbles)
- **Production-ready**: README marks the plugin as production-ready (no "test version" banner)

### 1.1.8

- _No separate npm release; changes rolled into 1.1.9._

### 1.1.7

- **Volume Control**: System volume from HomeKit (Lightbulb/Brightness slider)
  - Individual slider: Client "Enable Volume Slider" → each device gets its own volume accessory
  - Master Volume: Client "Join Master Volume" + plugin "Enable Global Volume" → one slider controls all devices in group
  - Mutual exclusion: enabling one disables the other
  - Real-time sync: when volume changed locally (keyboard) on any device in master group, plugin broadcasts to all others
  - Client `/volume?level=[0-100]` endpoint; macOS osascript, Windows PowerShell+COM, Linux pactl/amixer
  - Client Settings: Volume Slider Name (default: `[Hostname] - Volume`)
- **Screensaver Sync**: Start screensaver on all computers from HomeKit
  - Client Settings: "Enable Remote Screensaver" checkbox
  - Client `/screensaver` endpoint: macOS `ScreenSaverEngine` (several `open`/path/bundle fallbacks), Windows user `.scr` from registry, Linux `xdg-screensaver activate` (fallback: xscreensaver-command)
  - Plugin config: "Enable Global Screensaver Switch" → creates "All Screensavers" in HomeKit
  - Push button: ON sends to all online clients with screensaver enabled; auto-resets to OFF after 1.5s
- **Lock Computers**: Lock screen on all computers from HomeKit (does NOT put to sleep)
  - Client Settings: "Enable Remote Lock" checkbox
  - Client `/lock` endpoint: macOS osascript / Finder / `pmset displaysleepnow` fallback; Windows `rundll32 user32.dll,LockWorkStation`; Linux `xdg-screensaver lock`
  - Plugin config: "Enable Global Lock Switch" → creates "Lock Computers" in HomeKit
  - Push button: ON sends to all online clients with lock enabled; auto-resets to OFF after 1.5s
- **Managed Apps**: Quit mode names - Standard Quit (Graceful), Force Quit (Immediate), Smart Quit (Wait then Kill)
- **Managed Apps**: Skip WoL when computer already online (2s reachability check before wake sequence)
- **Managed Apps**: Add modal UX - enlarged dialog, Entry + filterable List, auto-focus

### 1.1.6

- **Managed Apps - Live Application Monitoring**: Real-state app control; Quit/Kill/Quit+Wait Kill; Wake Before Launch; Sleep After Quit

### 1.1.5

- **Wake Before Action**: Per-action option to wake the computer before running (WoL → 5s delay → wake-screen → run-action). Same flow as standard wake; display wakes from Dark Wake.
- **Sleep After Action**: Per-action option to put the computer to sleep 5 seconds after the action triggers. macOS: `pmset sleepnow` (osascript fallback). Stored as `wakeBefore` and `sleepAfter` in `actions.json`.
- **Config defaults**: Custom UI merges missing defaults when config is minimal (name, registrationPort, etc.).

### 1.1.4

- **Custom Actions**: Define actions in the client (BTT, shell, batch, AppleScript, URL) - they appear as HomeKit switches or tap buttons
  - Client Actions tab: add/delete actions; `actions.json` storage; heartbeat includes actions
  - Toggle (remembers state) or Push Button (one-shot); `{status}` substitution for on/off
  - BTT: full path to bttcli; args passed correctly (no shell wrapping)
  - AppleScript: file path (`.applescript`/`.scpt`) or inline script; `~` expansion
  - URL: HTTP request (async) or open in default browser
- **Platform-specific actions**: macOS (all); Linux (shell, batch, url); Windows (batch, url only)
- **Client UI**: Larger main window (620×720); Delete button blue/white; action name hint (no auto hostname prefix)
- **Example scripts**: `examples/` folder with example.sh, example.bat, example.applescript
- **Plugin fix**: `@homebridge/plugin-ui-utils` in dependencies + `homebridge-ui` in files - Config UI opens correctly on Node 20

### 1.1.2 / 1.1.3

- Plugin config did not open in Homebridge Config UI X (broken fix for Node 20 compatibility)

### 1.1.1

- **macOS Dark Mode**: Hide/Quit buttons use light text on dark window frame; sidebar, separators, and sidebar items follow system light/dark theme
- **Temperature Sensor**: Optional CPU temperature in HomeKit
  - Client checkbox "Send Temperature Data"; persisted in `client_config.json`
  - macOS: gopsutil sensors (SMC/HID), ioreg, system_profiler, powermetrics
  - Windows: WMI (MSAcpi_ThermalZoneTemperature, ThermalZoneInformation, Win32_TemperatureProbe)
  - Linux: sysfs thermal zones, `sensors` fallback
  - Dynamic add/remove of TemperatureSensor service based on client data
- **Windows**: HideWindow for exec (no console pop-ups); multi-WMI temperature fallbacks

### 1.1.0

- **Anti-Sleep device**: Virtual switch to prevent all computers from sleeping
  - Config: `antiSleepDeviceName` (default: "Computer Sleep Prevention"), `antiSleepTimer` (minutes, 0 = unlimited)
  - Siri: "Hey Siri, turn on [name]"
  - Client: `/stay-awake?enabled=true|false` - macOS: caffeinate -i, Windows: SetThreadExecutionState, Linux: systemd-inhibit
  - Client GUI: Anti-Sleep status indicator when active
- **Update notification**: Replaced auto-update with one-time notification; Download button opens link in browser
- **macOS client**: Zip distribution; Gatekeeper note (xattr -cr) in README and update dialog
- **Tray**: Left-click opens main window; About removed from menu

### 1.0.0

- **Group accessory**: Virtual "Computers" switch - Wake All / Sleep All (configurable name)
- **Auto-update**: Clients check version on heartbeat; download and self-update when plugin is newer
- **Token auth**: Plugin issues tokens on registration; client accepts only token-bearing requests (sleep, health, wake-screen)
- **macOS Dark Wake**: Uses `system_profiler SPDisplaysDataType` for Apple Silicon; `ioreg` fallback for Intel
- **Health check**: Plugin uses HTTP `/health` only (no ping); 10s timeout; treats `isDarkWake` as OFFLINE
- **Client port**: Default 45991 (was 8080) to avoid conflicts
- **Config UI**: Delete clients from list; inline confirmation (no `window.confirm`); badge contrast fix
- **HomeKit name**: When device display name changes, `updateDisplayName` + AccessoryInformation Name are updated
- **Auto-start**: Removed `launchctl load`; plist is only created; second instance is prevented
- **Response**: Removed raw fields from `displayState` (`ioregRaw`, `systemProfilerRaw`, `pmsetLastEvent`)

---

## Client Behavior

| Feature | Details |
|---|---|
| **Auto-Detection** | IP, MAC address, hostname detected on startup |
| **Heartbeat** | Sends registration on an interval (default 30s; configurable 5–300s in client Settings) |
| **Sleep (macOS)** | `pmset sleepnow` (osascript fallback) |
| **Sleep (Windows)** | `rundll32.exe powrprof.dll,SetSuspendState 0,1,0` |
| **Sleep (Linux)** | `systemctl suspend` |
| **macOS Hidden** | `.app` with `LSUIElement=true` - no Dock icon, no terminal |
| **Windows Hidden** | Built with `-H windowsgui` - no console window |
| **Version** | `--version` flag prints version; GUI shows version in header, info form, and About |
| **Temperature** | Optional "Send Temperature Data" checkbox; persisted in `client_config.json`; only reads CPU temp when enabled (minimizes CPU load) |
| **Actions** | Define custom actions (BTT, shell, batch, AppleScript, URL) in Actions tab; stored in `actions.json`; appear as HomeKit switches/buttons |
| **Managed Apps** | Add process names in Managed Apps tab; stored in `managed_apps.json`; heartbeat sends `app_states` (name → running); each app becomes a real-state switch in HomeKit |
| **Remote Screensaver** | Settings checkbox "Enable Remote Screensaver"; when enabled, client responds to `/screensaver` and advertises `screensaverEnabled` in heartbeat |
| **Remote Lock** | Settings checkbox "Enable Remote Lock"; when enabled, client responds to `/lock` and advertises `lockEnabled` in heartbeat |
| **Volume Control** | "Enable Volume Slider" (individual) or "Join Master Volume" (group); mutually exclusive. Volume Slider Name for individual. Polls volume every 2s when in master group; notifies plugin on local change |
| **Anti-Sleep** | "Enable Anti-Sleep (individual)" or "Join Anti-Sleep (global)"; mutually exclusive. Same pattern as Volume. Client `/stay-awake` endpoint; macOS: caffeinate -i, Windows: SetThreadExecutionState, Linux: systemd-inhibit |
| **Lock Prevention** | "Enable Lock Prevention (individual)" or "Join Lock Prevention (global)"; mutually exclusive. Prevents screen dim and lock. Uses same stay-awake mechanism as Anti-Sleep |

### Temperature Sensor (Optional)

| Platform | Method |
|---|---|
| **Linux** | `/sys/class/thermal/thermal_zone*/temp` (prefer `x86_pkg_temp` or `cpu-thermal`); fallback: `sensors` (Package id 0 / Core 0) |
| **macOS** | gopsutil sensors (SMC on Intel, HID on Apple Silicon); fallback: ioreg, system_profiler, powermetrics |
| **Windows** | WMI `Win32_PerfFormattedData_Counters_ThermalZoneInformation` (HighPrecisionTemperature) |

Value sent in millidegree Celsius; plugin converts to °C (÷1000). When checkbox is off, no temperature is sent and the TemperatureSensor service is removed from the accessory.

### Managed Apps (Live App Monitoring)

| Aspect | Details |
|--------|---------|
| **Process detection** | Client uses `ps` (macOS/Linux) or `tasklist` (Windows) to list running user apps |
| **Naming** | macOS: clean names (e.g. `Safari`, `AnyDesk`); Windows: includes `.exe` (e.g. `AnyDesk.exe`) |
| **Matching** | Case-insensitive; Windows `.exe` handled automatically |
| **Heartbeat** | Before each heartbeat, client checks `IsProcessRunning()` for each managed app; sends `app_states: {AppName: true/false}` |
| **Launch** | macOS: `open -a "AppName"`; Windows: `start "" "AppName"`; Linux: `exec` |
| **Standard Quit (Graceful)** | macOS: `osascript quit app`; Windows: `taskkill` without /F; Linux: `killall` (SIGTERM) |
| **Force Quit (Immediate)** | Force-kill all processes matching the name (gopsutil) |
| **Smart Quit (Wait then Kill)** | Try graceful quit, wait 4s, then force kill if still running |

### macOS Sleep / Power Nap Mitigation

| Mechanism | Description |
|---|---|
| **Going to Sleep** | Client sends `POST /going-to-sleep` before sleeping so the device is set OFF immediately |
| **20s State Lock** | Plugin ignores all signals for 20 seconds after sleep; only Wake command or physical display open sets ONLINE |
| **Heartbeat Filter** | Client checks display state before each heartbeat; if Dark Wake (display asleep), never sends |
| **Apple Silicon** | Uses `system_profiler SPDisplaysDataType` ("Display Asleep: Yes/No"); `ioreg` no longer exposes power state |
| **Full Wake** | `/wake-screen` runs caffeinate + key code 123 (user-active signal) + brightness max |

### Windows Notes

- **Status check**: Uses HTTP `/health` only (ping removed). Ensure client port (45991) is reachable from Homebridge.
- **Sleep**: If sleep commands fail, ensure Windows Firewall allows incoming connections on the client port (default 45991), and that the Homebridge host can reach the client's IP (e.g. same subnet or proper routing).

## Plugin API Endpoints (Port 9090)

| Method | Path | Description |
|---|---|---|
| `POST` | `/register` | Client heartbeat/registration (returns token; may include update info) |
| `POST` | `/going-to-sleep` | Client notifies before sleeping (body: `{"mac":"..."}`) |
| `POST` | `/volume-changed` | Client notifies when volume changed locally (body: `{"mac":"...","level":0-100}`); plugin broadcasts to other devices in master group |
| `GET` | `/clients` | List all registered clients |
| `DELETE` | `/clients/:mac` | Remove a client |
| `GET` | `/download` | List available client binaries |
| `GET` | `/download/:platform` | Download a client binary |

## Client Endpoints (Port 45991)

All client endpoints require `X-Auth-Token` header (issued by plugin on registration).

| Method | Path | Description |
|---|---|---|
| `GET` | `/health` | Liveness + `isDarkWake` (plugin uses for ONLINE/OFFLINE) |
| `GET` | `/status` | Hostname, uptime, display state |
| `POST` | `/sleep` | Put computer to sleep |
| `POST` | `/wake-screen` | Force display wake (macOS: caffeinate + key + brightness) |
| `GET` | `/stay-awake?enabled=true\|false` | Enable/disable system sleep prevention (Anti-Sleep + Lock Prevention) |
| `GET` | `/run-action?name=X&state=on\|off` | Execute a named action (BTT, shell, batch, AppleScript, URL) |
| `GET` | `/manage-app?name=X&target=on\|off` | Launch app (target=on) or quit/kill (target=off) based on app's QuitMode |
| `GET` | `/screensaver` | Start screensaver (requires Enable Remote Screensaver in client) |
| `GET` | `/lock` | Lock screen (requires Enable Remote Lock in client; does not sleep) |
| `GET` | `/volume` | Get current volume (no params) or set volume (`?level=0-100`; requires Enable Volume Slider or Join Master Volume) |

## License
MIT

## Developer

Orkan K.
