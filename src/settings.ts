import * as pkg from '../package.json';

/**
 * Platform name and plugin identifier used for Homebridge registration.
 */

export const PLATFORM_NAME = 'ComputerControl';
export const CLIENT_VERSION = (pkg as { version?: string }).version || '1.0.0';
export const PLUGIN_NAME = 'homebridge-computer-control';

/**
 * Default port for the registration HTTP server on the plugin side.
 */
export const REGISTRATION_PORT = 9090;

/**
 * Default port for the client HTTP server.
 */
export const DEFAULT_CLIENT_PORT = 45991;

/**
 * Interval (ms) for checking if a registered client is online.
 */
export const STATUS_CHECK_INTERVAL = 15_000; // 15 seconds

/**
 * Timeout (ms) for ping/health checks.
 */
export const PING_TIMEOUT = 10; // 10 seconds

/**
 * Seconds to ignore all signals after "going to sleep" (State Lock).
 * Only accept ONLINE when user physically opens display or Wake command is sent.
 */
export const SLEEP_DEBOUNCE_SECONDS = 20;

/**
 * Represents a registered computer client.
 */
export interface RegisteredClient {
  hostname: string;
  ip: string;
  mac: string;
  port: number;
  os: string;
  lastSeen: number; // Unix timestamp (ms)
  displayName?: string;
  isDarkWake?: boolean; // macOS: Power Nap, plugin keeps device OFF
  token?: string; // Auth token for client requests (sleep, wake-screen, health)
  /** CPU temperature in millidegree Celsius (÷1000 = °C); only when client sends it */
  temperature?: number | null;
  /** Custom actions from client (HomeKit switches/buttons) */
  actions?: ClientAction[];
  /** Managed app name -> running (true/false). Keys are the managed app names. */
  appStates?: Record<string, boolean>;
  /** Managed app config with wakeBefore/sleepAfter. */
  managedApps?: ClientManagedApp[];
  /** Client has Enable Remote Screensaver checked. */
  screensaverEnabled?: boolean;
  /** Client has Enable Remote Lock checked. */
  lockEnabled?: boolean;
  /** Client has Enable Volume Slider checked. */
  enableVolumeSlider?: boolean;
  /** Client has Join Master Volume checked. */
  joinMasterVolume?: boolean;
  /** Custom name for this device's volume slider (default: [Hostname] - Volume). */
  volumeSliderName?: string;
  /** Current volume level 0-100 from heartbeat (for status sync). */
  volume?: number | null;
  /** Client has Enable Anti-Sleep (individual) checked. */
  enableAntiSleep?: boolean;
  /** Client has Join Anti-Sleep (global) checked. */
  joinAntiSleep?: boolean;
  /** Client has Enable Lock Prevention (individual) checked. */
  enableLockPrevention?: boolean;
  /** Client has Join Lock Prevention (global) checked. */
  joinLockPrevention?: boolean;
}

/**
 * Custom action from client (BTT, shell, URL, etc.).
 */
export interface ClientAction {
  name: string;
  type: string; // btt_trigger, shell, batch, applescript, url
  value: string;
  interface: string; // toggle or button
  urlMode?: string; // fetch or browser, only when type is url
  wakeBefore?: boolean;
  sleepAfter?: boolean;
}

/**
 * Managed app from client (live app monitoring).
 * App name as stored on client; state comes from appStates.
 */
export interface ClientManagedApp {
  name: string;
  displayName?: string; // Optional custom name shown in HomeKit (e.g. "Firefox" instead of "firefox.exe")
  wakeBefore?: boolean;
  sleepAfter?: boolean;
}

/**
 * Plugin configuration from config.json.
 */
export interface ComputerControlConfig {
  platform: string;
  name?: string;
  registrationPort?: number;
  groupAccessoryName?: string;
  antiSleepDeviceName?: string;
  antiSleepTimer?: number;
  clients?: RegisteredClient[];
  /** Enable Global Volume accessory (Master Slider). When on, all clients with Join Master Volume get same level. */
  enableGlobalVolumeSwitch?: boolean;
  /** Display name for the Global Volume accessory (default: Computer Volume). */
  masterVolumeName?: string;
  /** Enable Global Anti-Sleep accessory. Only shown when clients with Join Anti-Sleep exist. */
  enableGlobalAntiSleepSwitch?: boolean;
  /** Enable Global Lock Prevention accessory. Only shown when clients with Join Lock Prevention exist. */
  enableGlobalLockPreventionSwitch?: boolean;
  /** Display name for the Lock Prevention accessory (default: Lock Prevention). */
  lockPreventionDeviceName?: string;
}
