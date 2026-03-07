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
}
