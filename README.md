# 🏠 HomeBridge Computer Control

[![npm version](https://img.shields.io/npm/v/homebridge-computer-control.svg)](https://www.npmjs.com/package/homebridge-computer-control)
[![npm downloads](https://img.shields.io/npm/dm/homebridge-computer-control.svg)](https://www.npmjs.com/package/homebridge-computer-control)
[![License: MIT](https://img.shields.io/github/license/orkank/homebridge-computer-control.svg)](https://opensource.org/licenses/MIT)
[![GitHub stars](https://img.shields.io/github/stars/orkank/homebridge-computer-control.svg)](https://github.com/orkank/homebridge-computer-control/stargazers)
[![GitHub forks](https://img.shields.io/github/forks/orkank/homebridge-computer-control.svg)](https://github.com/orkank/homebridge-computer-control/network)

> ⚠️ **Test version** — This plugin is still in testing.

Control your computers (macOS, Windows, Linux) through Apple HomeKit using Homebridge. Wake them with WoL, put them to sleep remotely, and manage them as HomeKit switches.

**Version:** 1.1.0

## Features

| Feature | Description |
|---------|-------------|
| **Wake-on-LAN** | Wake sleeping computers from anywhere via HomeKit |
| **Remote Sleep** | Put computers to sleep with a single tap |
| **Group Control** | Virtual "Computers" accessory — Wake All / Sleep All in one command |
| **Auto-Registration** | Clients register automatically; no manual config needed |
| **Update Notification** | Clients receive a one-time update notification with download link |
| **macOS Power Nap** | Correctly detects Dark Wake; device stays OFF when display is asleep |
| **Token Auth** | Client and plugin use shared tokens; no unauthorized sleep/wake |
| **Config UI** | View clients, remove stale ones, configure group name |
| **Anti-Sleep** | Virtual switch to prevent all computers from sleeping (configurable name + optional timer) |

## Architecture

```
┌──────────────────────────────────────────────────────────┐
│                    Apple Home / HomeKit                  │
└────────────────────────────┬─────────────────────────────┘
                             │
┌────────────────────────────▼─────────────────────────────┐
│                   Homebridge Plugin                      │
│  ┌─────────────┐  ┌──────────────┐  ┌─────────────────┐ │
│  │ Registration│  │ Wake-on-LAN  │  │  Status Check   │ │
│  │   Server    │  │   (Power On) │  │  (HTTP)         │ │
│  └─────────────┘  └──────────────┘  └─────────────────┘ │
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
Add to your `config.json` (or use Config UI X):
```json
{
  "platforms": [
    {
      "platform": "ComputerControl",
      "name": "Computer Control",
      "registrationPort": 9090
    }
  ]
}
```

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

### 1.1.0 (Current)

- **Anti-Sleep device**: Virtual switch to prevent all computers from sleeping
  - Config: `antiSleepDeviceName` (default: "Computer Sleep Prevention"), `antiSleepTimer` (minutes, 0 = unlimited)
  - Siri: "Hey Siri, turn on [name]"
  - Client: `/stay-awake?enabled=true|false` — macOS: caffeinate -i, Windows: SetThreadExecutionState, Linux: systemd-inhibit
  - Client GUI: Anti-Sleep status indicator when active
- **Update notification**: Replaced auto-update with one-time notification; Download button opens link in browser
- **macOS client**: Zip distribution; Gatekeeper note (xattr -cr) in README and update dialog
- **Tray**: Left-click opens main window; About removed from menu

### 1.0.0

- **Group accessory**: Virtual "Computers" switch — Wake All / Sleep All (configurable name)
- **Auto-update**: Clients check version on heartbeat; download and self-update when plugin is newer
- **Token auth**: Plugin issues tokens on registration; client accepts only token-bearing requests (sleep, health, wake-screen)
- **macOS Dark Wake**: Uses `system_profiler SPDisplaysDataType` for Apple Silicon; `ioreg` fallback for Intel
- **Health check**: Plugin uses HTTP `/health` only (no ping); 10s timeout; treats `isDarkWake` as OFFLINE
- **Client port**: Default 45991 (was 8080) to avoid conflicts
- **Config UI**: Delete clients from list; inline confirmation (no `window.confirm`); badge contrast fix
- **HomeKit name**: When device display name changes, `updateDisplayName` + AccessoryInformation Name are updated
- **macOS tray**: Exit menu item removed (Quit remains in app menu)
- **Auto-start**: Removed `launchctl load`; plist is only created; second instance is prevented
- **Response**: Removed raw fields from `displayState` (`ioregRaw`, `systemProfilerRaw`, `pmsetLastEvent`)

---

## Client Behavior

| Feature | Details |
|---|---|
| **Auto-Detection** | IP, MAC address, hostname detected on startup |
| **Heartbeat** | Sends registration every 30 seconds |
| **Sleep (macOS)** | `osascript -e 'tell application "System Events" to sleep'` |
| **Sleep (Windows)** | `rundll32.exe powrprof.dll,SetSuspendState 0,1,0` |
| **Sleep (Linux)** | `systemctl suspend` |
| **macOS Hidden** | `.app` with `LSUIElement=true` — no Dock icon, no terminal |
| **Windows Hidden** | Built with `-H windowsgui` — no console window |
| **Version** | `--version` flag prints version; GUI shows version in header, info form, and About |

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
| `GET` | `/stay-awake?enabled=true\|false` | Enable/disable system sleep prevention (Anti-Sleep) |

## Publishing (Maintainers)

```bash
# 1. Build plugin + client binaries (bin/ required for download server)
npm run build:all

# 2. Publish to npm
npm publish
```

## License
MIT

## Developer

Orkan K.
