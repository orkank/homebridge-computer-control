import {
  Service,
  PlatformAccessory,
  CharacteristicValue,
} from 'homebridge';

import { ComputerControlPlatform } from './platform';
import { RegisteredClient, ClientAction } from './settings';

export const GROUP_ACCESSORY_UUID = 'computer-control-group';
export const ANTI_SLEEP_ACCESSORY_UUID = 'computer-control-anti-sleep';
export const SCREENSAVER_ACCESSORY_UUID = 'computer-control-all-screensavers';
export const LOCK_ACCESSORY_UUID = 'computer-control-lock-computers';
export const GLOBAL_VOLUME_ACCESSORY_UUID = 'computer-control-global-volume';

/**
 * GroupComputerAccessory
 *
 * Virtual accessory that forwards Wake/Sleep commands to ALL registered clients.
 * - ON  = Wake all computers (WoL + wake-screen)
 * - OFF = Sleep all computers
 * - Status: ON if at least one client is online
 */
export class GroupComputerAccessory {
  private service: Service;
  private isAnyOnline = false;

  constructor(
    private readonly platform: ComputerControlPlatform,
    private readonly accessory: PlatformAccessory,
    displayName: string,
  ) {
    this.accessory
      .getService(this.platform.Service.AccessoryInformation)!
      .setCharacteristic(this.platform.Characteristic.Name, displayName)
      .setCharacteristic(this.platform.Characteristic.Manufacturer, 'HomeBridge Computer Control')
      .setCharacteristic(this.platform.Characteristic.Model, 'Group')
      .setCharacteristic(this.platform.Characteristic.SerialNumber, GROUP_ACCESSORY_UUID);

    this.service =
      this.accessory.getService(this.platform.Service.Switch) ||
      this.accessory.addService(this.platform.Service.Switch);

    this.service.setCharacteristic(this.platform.Characteristic.Name, displayName);

    this.service
      .getCharacteristic(this.platform.Characteristic.On)
      .onGet(this.handleOnGet.bind(this))
      .onSet(this.handleOnSet.bind(this));

    this.checkOnlineStatus();
  }

  public updateDisplayName(name: string): void {
    this.service.setCharacteristic(this.platform.Characteristic.Name, name);
    this.accessory.updateDisplayName(name);
    const infoService = this.accessory.getService(this.platform.Service.AccessoryInformation);
    if (infoService) {
      infoService.updateCharacteristic(this.platform.Characteristic.Name, name);
    }
  }

  private handleOnGet(): CharacteristicValue {
    return this.isAnyOnline;
  }

  private async handleOnSet(value: CharacteristicValue): Promise<void> {
    const targetState = value as boolean;
    const clients = this.platform.getClients();

    if (clients.length === 0) {
      this.platform.log.info('📭 No clients registered — group command ignored');
      return;
    }

    if (targetState) {
      // ──── WAKE ALL ────
      this.platform.log.info(`⏰ Waking all computers (${clients.length} devices)`);

      for (const client of clients) {
        try {
          await this.platform.sendWakeOnLan(client.mac);
          this.platform.log.debug(`✅ WoL sent to ${client.hostname}`);
        } catch (err) {
          this.platform.log.warn(`⚠️ WoL failed for ${client.hostname}: ${(err as Error).message}`);
        }
      }

      await new Promise((r) => setTimeout(r, 5000)); // 5s for deep sleep wake

      for (const client of clients) {
        const ok = await this.platform.sendWakeScreenRequest(client.ip, client.port, client.token);
        if (ok) {
          this.platform.log.debug(`✅ Wake-screen sent to ${client.hostname}`);
        }
      }

      setTimeout(() => this.checkOnlineStatus(), 10_000);
    } else {
      // ──── SLEEP ALL ────
      this.platform.log.info(`💤 Putting all computers to sleep (${clients.length} devices)`);

      for (const client of clients) {
        const success = await this.platform.sendSleepRequest(client.ip, client.port, client.token);
        if (success) {
          this.platform.log.debug(`✅ Sleep sent to ${client.hostname}`);
        } else {
          this.platform.log.warn(`⚠️ Sleep may have failed for ${client.hostname}`);
        }
      }

      this.isAnyOnline = false;
      this.service.updateCharacteristic(this.platform.Characteristic.On, false);
    }
  }

  public async checkOnlineStatus(): Promise<void> {
    const clients = this.platform.getClients();

    if (clients.length === 0) {
      this.isAnyOnline = false;
      this.service.updateCharacteristic(this.platform.Characteristic.On, false);
      return;
    }

    let anyOnline = false;
    for (const client of clients) {
      const macKey = client.mac.toUpperCase();
      if (this.platform.isInSleepDebounceWindow(macKey)) {
        continue;
      }
      const alive = await this.platform.httpHealthCheck(client.ip, client.port, client.token);
      if (alive) {
        anyOnline = true;
        break;
      }
    }

    this.isAnyOnline = anyOnline;
    this.service.updateCharacteristic(this.platform.Characteristic.On, anyOnline);
  }
}

/**
 * AntiSleepAccessory
 *
 * Virtual switch that prevents all computers from sleeping.
 * - ON  = Send /stay-awake?enabled=true to all active clients
 * - OFF = Send /stay-awake?enabled=false to all clients
 * - If antiSleepTimer > 0: auto-turn-off after N minutes
 */
export class AntiSleepAccessory {
  private service: Service;
  private isOn = false;
  private timerHandle: ReturnType<typeof setTimeout> | null = null;

  constructor(
    private readonly platform: ComputerControlPlatform,
    private readonly accessory: PlatformAccessory,
    displayName: string,
  ) {
    this.accessory
      .getService(this.platform.Service.AccessoryInformation)!
      .setCharacteristic(this.platform.Characteristic.Name, displayName)
      .setCharacteristic(this.platform.Characteristic.Manufacturer, 'HomeBridge Computer Control')
      .setCharacteristic(this.platform.Characteristic.Model, 'Anti-Sleep')
      .setCharacteristic(this.platform.Characteristic.SerialNumber, ANTI_SLEEP_ACCESSORY_UUID);

    this.service =
      this.accessory.getService(this.platform.Service.Switch) ||
      this.accessory.addService(this.platform.Service.Switch);

    this.service.setCharacteristic(this.platform.Characteristic.Name, displayName);

    this.service
      .getCharacteristic(this.platform.Characteristic.On)
      .onGet(this.handleOnGet.bind(this))
      .onSet(this.handleOnSet.bind(this));
  }

  public updateDisplayName(name: string): void {
    this.service.setCharacteristic(this.platform.Characteristic.Name, name);
    this.accessory.updateDisplayName(name);
    const infoService = this.accessory.getService(this.platform.Service.AccessoryInformation);
    if (infoService) {
      infoService.updateCharacteristic(this.platform.Characteristic.Name, name);
    }
  }

  private handleOnGet(): CharacteristicValue {
    return this.isOn;
  }

  private async handleOnSet(value: CharacteristicValue): Promise<void> {
    const targetState = value as boolean;

    this.clearTimer();

    if (targetState) {
      this.isOn = true;
      this.service.updateCharacteristic(this.platform.Characteristic.On, true);
      this.platform.log.info('☕ Anti-Sleep ON — preventing all computers from sleeping');

      await this.platform.sendStayAwakeToAllClients(true);

      const timerMinutes = this.platform.getAntiSleepTimer();
      if (timerMinutes > 0) {
        this.timerHandle = setTimeout(() => {
          this.timerHandle = null;
          this.platform.log.info(`⏱️ Anti-Sleep timer expired (${timerMinutes} min) — turning OFF`);
          this.isOn = false;
          this.service.updateCharacteristic(this.platform.Characteristic.On, false);
          this.platform.sendStayAwakeToAllClients(false);
        }, timerMinutes * 60 * 1000);
      }
    } else {
      this.isOn = false;
      this.service.updateCharacteristic(this.platform.Characteristic.On, false);
      this.platform.log.info('☕ Anti-Sleep OFF');

      await this.platform.sendStayAwakeToAllClients(false);
    }
  }

  private clearTimer(): void {
    if (this.timerHandle) {
      clearTimeout(this.timerHandle);
      this.timerHandle = null;
    }
  }
}

/**
 * AllScreensaversAccessory
 *
 * Push-button switch: ON sends screensaver to all clients with screensaverEnabled.
 * Auto-resets to OFF after 1.5 seconds (screensaver state is hard to track).
 */
export class AllScreensaversAccessory {
  private service: Service;

  constructor(
    private readonly platform: ComputerControlPlatform,
    private readonly accessory: PlatformAccessory,
    displayName: string,
  ) {
    this.accessory
      .getService(this.platform.Service.AccessoryInformation)!
      .setCharacteristic(this.platform.Characteristic.Name, displayName)
      .setCharacteristic(this.platform.Characteristic.Manufacturer, 'HomeBridge Computer Control')
      .setCharacteristic(this.platform.Characteristic.Model, 'Screensaver Sync')
      .setCharacteristic(this.platform.Characteristic.SerialNumber, SCREENSAVER_ACCESSORY_UUID);

    this.service =
      this.accessory.getService(this.platform.Service.Switch) ||
      this.accessory.addService(this.platform.Service.Switch);

    this.service.setCharacteristic(this.platform.Characteristic.Name, displayName);

    this.service
      .getCharacteristic(this.platform.Characteristic.On)
      .onGet(() => false)
      .onSet(this.handleOnSet.bind(this));
  }

  private async handleOnSet(value: CharacteristicValue): Promise<void> {
    if (!(value as boolean)) {
      return;
    }

    this.platform.log.info('🖼️ All Screensavers — sending to enabled clients');
    await this.platform.sendScreensaverToAllClients();

    // Push button: stay ON 1.5s for feedback, then reset to OFF
    setTimeout(() => {
      this.service.updateCharacteristic(this.platform.Characteristic.On, false);
    }, 1500);
  }
}

/**
 * LockComputersAccessory
 *
 * Push-button switch: ON locks screen on all online clients with lockEnabled.
 * Does NOT put computers to sleep. Same logic as All Screensavers.
 */
export class LockComputersAccessory {
  private service: Service;

  constructor(
    private readonly platform: ComputerControlPlatform,
    private readonly accessory: PlatformAccessory,
    displayName: string,
  ) {
    this.accessory
      .getService(this.platform.Service.AccessoryInformation)!
      .setCharacteristic(this.platform.Characteristic.Name, displayName)
      .setCharacteristic(this.platform.Characteristic.Manufacturer, 'HomeBridge Computer Control')
      .setCharacteristic(this.platform.Characteristic.Model, 'Lock Computers')
      .setCharacteristic(this.platform.Characteristic.SerialNumber, LOCK_ACCESSORY_UUID);

    this.service =
      this.accessory.getService(this.platform.Service.Switch) ||
      this.accessory.addService(this.platform.Service.Switch);

    this.service.setCharacteristic(this.platform.Characteristic.Name, displayName);

    this.service
      .getCharacteristic(this.platform.Characteristic.On)
      .onGet(() => false)
      .onSet(this.handleOnSet.bind(this));
  }

  private async handleOnSet(value: CharacteristicValue): Promise<void> {
    if (!(value as boolean)) {
      return;
    }

    this.platform.log.info('🔒 Lock Computers — sending to online enabled clients');
    await this.platform.sendLockToAllClients();

    setTimeout(() => {
      this.service.updateCharacteristic(this.platform.Characteristic.On, false);
    }, 1500);
  }
}

/**
 * VolumeAccessory
 *
 * Per-device volume slider using Lightbulb + Brightness (0-100 = volume level).
 * Display name: client.volumeSliderName or "[Hostname] - Volume".
 */
export class VolumeAccessory {
  public readonly accessory: PlatformAccessory;
  private service: Service;

  constructor(
    private readonly platform: ComputerControlPlatform,
    accessory: PlatformAccessory,
    private client: RegisteredClient,
  ) {
    this.accessory = accessory;
    const displayName = this.getDisplayName();

    this.accessory
      .getService(this.platform.Service.AccessoryInformation)!
      .setCharacteristic(this.platform.Characteristic.Name, displayName)
      .setCharacteristic(this.platform.Characteristic.Manufacturer, 'HomeBridge Computer Control')
      .setCharacteristic(this.platform.Characteristic.Model, 'Volume')
      .setCharacteristic(this.platform.Characteristic.SerialNumber, `volume-${client.mac}`);

    this.service =
      this.accessory.getService(this.platform.Service.Lightbulb) ||
      this.accessory.addService(this.platform.Service.Lightbulb);

    this.service.setCharacteristic(this.platform.Characteristic.Name, displayName);

    this.service
      .getCharacteristic(this.platform.Characteristic.On)
      .onGet(this.handleOnGet.bind(this))
      .onSet(this.handleOnSet.bind(this));

    this.service
      .getCharacteristic(this.platform.Characteristic.Brightness)
      .onGet(this.handleBrightnessGet.bind(this))
      .onSet(this.handleBrightnessSet.bind(this));

    this.syncVolumeToCharacteristics();
  }

  private getDisplayName(): string {
    const name = (this.client.volumeSliderName || '').trim();
    if (name) return name;
    return `${this.client.hostname} - Volume`;
  }

  public updateClient(client: RegisteredClient): void {
    this.client = client;
    const displayName = this.getDisplayName();
    this.service.setCharacteristic(this.platform.Characteristic.Name, displayName);
    this.syncVolumeToCharacteristics();
  }

  private syncVolumeToCharacteristics(): void {
    const v = this.client.volume;
    const level = typeof v === 'number' && v >= 0 && v <= 100 ? v : 50;
    this.service.updateCharacteristic(this.platform.Characteristic.Brightness, level);
    this.service.updateCharacteristic(this.platform.Characteristic.On, level > 0);
  }

  private handleOnGet(): CharacteristicValue {
    const v = this.client.volume;
    if (typeof v === 'number' && v >= 0 && v <= 100) return v > 0;
    return true;
  }

  private handleOnSet(value: CharacteristicValue): void {
    if (value as boolean) {
      // Turn on: if volume is 0, set to 50
      if (this.client.volume === 0) {
        this.client.volume = 50;
        this.syncVolumeToCharacteristics();
        this.platform.sendVolumeRequest(this.client.ip, this.client.port, 50, this.client.token);
      }
      return;
    }
    const level = 0;
    this.client.volume = level;
    this.syncVolumeToCharacteristics();
    this.platform.sendVolumeRequest(this.client.ip, this.client.port, level, this.client.token);
  }

  private handleBrightnessGet(): CharacteristicValue {
    const v = this.client.volume;
    if (typeof v === 'number' && v >= 0 && v <= 100) return v;
    return 50;
  }

  private async handleBrightnessSet(value: CharacteristicValue): Promise<void> {
    const level = Math.round(Math.min(100, Math.max(0, value as number)));
    const ok = await this.platform.sendVolumeRequest(
      this.client.ip,
      this.client.port,
      level,
      this.client.token,
    );
    if (ok) {
      this.client.volume = level;
      this.syncVolumeToCharacteristics();
    }
  }
}

/**
 * GlobalVolumeAccessory
 *
 * Master volume slider: sets same level on all clients with Join Master Volume.
 * Uses Lightbulb + Brightness. Display name from config.masterVolumeName (default: Computer Volume).
 */
export class GlobalVolumeAccessory {
  private service: Service;
  /** Last set value — never wait for client responses; return this to HomeKit. */
  private lastSetLevel = 50;

  constructor(
    private readonly platform: ComputerControlPlatform,
    private readonly accessory: PlatformAccessory,
    displayName: string,
  ) {
    this.accessory
      .getService(this.platform.Service.AccessoryInformation)!
      .setCharacteristic(this.platform.Characteristic.Name, displayName)
      .setCharacteristic(this.platform.Characteristic.Manufacturer, 'HomeBridge Computer Control')
      .setCharacteristic(this.platform.Characteristic.Model, 'Global Volume')
      .setCharacteristic(this.platform.Characteristic.SerialNumber, GLOBAL_VOLUME_ACCESSORY_UUID);

    this.service =
      this.accessory.getService(this.platform.Service.Lightbulb) ||
      this.accessory.addService(this.platform.Service.Lightbulb);

    this.service.setCharacteristic(this.platform.Characteristic.Name, displayName);

    this.service
      .getCharacteristic(this.platform.Characteristic.On)
      .onGet(this.handleOnGet.bind(this))
      .onSet(this.handleOnSet.bind(this));

    this.service
      .getCharacteristic(this.platform.Characteristic.Brightness)
      .onGet(this.handleBrightnessGet.bind(this))
      .onSet(this.handleBrightnessSet.bind(this));

    const avg = this.getAverageVolume();
    if (avg >= 0) {
      this.lastSetLevel = avg;
      this.syncVolumeToCharacteristics();
    }
  }

  private syncVolumeToCharacteristics(): void {
    this.service.updateCharacteristic(this.platform.Characteristic.Brightness, this.lastSetLevel);
    this.service.updateCharacteristic(this.platform.Characteristic.On, this.lastSetLevel > 0);
  }

  private handleOnGet(): CharacteristicValue {
    return this.lastSetLevel > 0;
  }

  private handleOnSet(value: CharacteristicValue): void {
    if (value as boolean) {
      if (this.lastSetLevel === 0) {
        this.lastSetLevel = 50;
        this.syncVolumeToCharacteristics();
        this.platform.sendVolumeToAllClients(50);
      }
      return;
    }
    this.lastSetLevel = 0;
    this.syncVolumeToCharacteristics();
    this.platform.sendVolumeToAllClients(0);
  }

  public updateDisplayName(name: string): void {
    this.service.setCharacteristic(this.platform.Characteristic.Name, name);
    this.accessory.updateDisplayName(name);
    const infoService = this.accessory.getService(this.platform.Service.AccessoryInformation);
    if (infoService) {
      infoService.updateCharacteristic(this.platform.Characteristic.Name, name);
    }
  }

  public updateVolumeFromHeartbeat(): void {
    const avg = this.getAverageVolume();
    if (avg >= 0) {
      this.lastSetLevel = avg;
      this.syncVolumeToCharacteristics();
    }
  }

  /** Set volume level directly (e.g. from volume-changed); no client query. */
  public updateVolumeLevel(level: number): void {
    this.lastSetLevel = Math.round(Math.min(100, Math.max(0, level)));
    this.syncVolumeToCharacteristics();
  }

  private getAverageVolume(): number {
    const clients = this.platform.getClientsWithJoinMasterVolume();
    if (clients.length === 0) return -1;
    let sum = 0;
    let count = 0;
    for (const c of clients) {
      const v = c.volume;
      if (typeof v === 'number' && v >= 0 && v <= 100) {
        sum += v;
        count++;
      }
    }
    if (count === 0) return -1;
    return Math.round(sum / count);
  }

  private handleBrightnessGet(): CharacteristicValue {
    return this.lastSetLevel;
  }

  private handleBrightnessSet(value: CharacteristicValue): void {
    const level = Math.round(Math.min(100, Math.max(0, value as number)));
    this.lastSetLevel = level;
    this.syncVolumeToCharacteristics();
    this.platform.sendVolumeToAllClients(level);
  }
}

/**
 * ComputerAccessory
 *
 * Handles individual computer accessories in HomeKit.
 * Each accessory is a Switch that:
 * - ON  = Wake the computer (Wake-on-LAN)
 * - OFF = Sleep the computer (HTTP request to client)
 *
 * Status is determined by pinging the client.
 */
const TEMPERATURE_SERVICE_SUBTYPE = 'cpu-temperature';

/**
 * ActionAccessory
 *
 * Custom action from client (BTT, shell, URL, etc.).
 * - Toggle: State memory — last HomeKit state is remembered, no polling.
 * - Button: Single tap fires "on", then resets to OFF.
 */
export class ActionAccessory {
  public readonly accessory: PlatformAccessory;
  private service: Service;
  private lastState = false; // For toggle: remembered state

  constructor(
    private readonly platform: ComputerControlPlatform,
    accessory: PlatformAccessory,
    private client: RegisteredClient,
    private action: ClientAction,
  ) {
    this.accessory = accessory;

    this.accessory
      .getService(this.platform.Service.AccessoryInformation)!
      .setCharacteristic(this.platform.Characteristic.Name, action.name)
      .setCharacteristic(this.platform.Characteristic.Manufacturer, 'HomeBridge Computer Control')
      .setCharacteristic(this.platform.Characteristic.Model, 'Action')
      .setCharacteristic(this.platform.Characteristic.SerialNumber, `action-${client.mac}-${action.name}`);

    this.service =
      this.accessory.getService(this.platform.Service.Switch) ||
      this.accessory.addService(this.platform.Service.Switch);

    this.service.setCharacteristic(this.platform.Characteristic.Name, action.name);

    this.service
      .getCharacteristic(this.platform.Characteristic.On)
      .onGet(this.handleOnGet.bind(this))
      .onSet(this.handleOnSet.bind(this));

    // Initial state
    this.service.updateCharacteristic(this.platform.Characteristic.On, this.lastState);
  }

  public updateAction(action: ClientAction): void {
    this.action = action;
    this.service.setCharacteristic(this.platform.Characteristic.Name, action.name);
  }

  private handleOnGet(): CharacteristicValue {
    return this.lastState;
  }

  private async handleOnSet(value: CharacteristicValue): Promise<void> {
    const targetState = value as boolean;

    if (this.action.interface === 'button') {
      // Push Button: only react to ON (user tap). Ignore OFF — we auto-reset, never send off to client.
      if (!targetState) {
        return; // User or our auto-reset set OFF — no command, no loop
      }
      this.runActionWithWake('on');
      this.platform.log.info(`🔘 Action button: ${this.action.name} (${this.client.hostname})`);
      // Auto-reset: programmatically set OFF after 500ms. updateCharacteristic does NOT trigger onSet (no loop).
      setTimeout(() => {
        this.lastState = false;
        this.service.updateCharacteristic(this.platform.Characteristic.On, false);
      }, 500);
    } else {
      // Toggle: state memory — remember last state, fire run-action
      this.lastState = targetState;
      const state = targetState ? 'on' : 'off';
      this.runActionWithWake(state);
      this.platform.log.info(`🔘 Action toggle: ${this.action.name} = ${state} (${this.client.hostname})`);
    }
  }

  private async runActionWithWake(state: 'on' | 'off'): Promise<void> {
    // Same flow as standard wake: WoL -> 5s delay -> wake-screen (full display) -> run-action
    if (this.action.wakeBefore && this.client.mac) {
      this.platform.log.info(`⏰ Wake before action: ${this.client.hostname}`);
      await this.platform.sendWakeOnLan(this.client.mac);
      await new Promise((r) => setTimeout(r, 5000)); // 5s for deep sleep wake (same as standard wake)
      const wakeScreenOk = await this.platform.sendWakeScreenRequest(
        this.client.ip,
        this.client.port,
        this.client.token,
      );
      if (wakeScreenOk) {
        this.platform.log.info(`✅ Wake-screen sent (display on): ${this.client.hostname}`);
      } else {
        this.platform.log.debug(`⚠️  Wake-screen failed for ${this.client.hostname} (may be non-macOS or not yet online)`);
      }
    }
    await this.platform.sendRunActionRequest(
      this.client.ip,
      this.client.port,
      this.action.name,
      state,
      this.client.token,
    );
  }
}

/**
 * ManagedAppAccessory
 *
 * Switch that reflects live app state from the client.
 * - ON  = app is running
 * - OFF = app is not running
 * - handleOnGet returns app_states[appName] (real state from client)
 * - handleOnSet: when user taps, call /manage-app?name=X&target=on or off
 * - CRITICAL: updateCharacteristic(On, value) ONLY from app_states — never from user action.
 */
export class ManagedAppAccessory {
  public readonly accessory: PlatformAccessory;
  private service: Service;

  constructor(
    private readonly platform: ComputerControlPlatform,
    accessory: PlatformAccessory,
    private client: RegisteredClient,
    private appName: string,
  ) {
    this.accessory = accessory;

    this.accessory
      .getService(this.platform.Service.AccessoryInformation)!
      .setCharacteristic(this.platform.Characteristic.Name, appName)
      .setCharacteristic(this.platform.Characteristic.Manufacturer, 'HomeBridge Computer Control')
      .setCharacteristic(this.platform.Characteristic.Model, 'Managed App')
      .setCharacteristic(this.platform.Characteristic.SerialNumber, `managed-app-${client.mac}-${appName}`);

    this.service =
      this.accessory.getService(this.platform.Service.Switch) ||
      this.accessory.addService(this.platform.Service.Switch);

    this.service.setCharacteristic(this.platform.Characteristic.Name, appName);

    this.service
      .getCharacteristic(this.platform.Characteristic.On)
      .onGet(this.handleOnGet.bind(this))
      .onSet(this.handleOnSet.bind(this));

    // Initial state from app_states
    this.updateFromAppStates(client.appStates);
  }

  /**
   * Update the On characteristic from app_states. ONLY source of truth for display.
   */
  public updateFromAppStates(appStates?: Record<string, boolean>): void {
    const running = !!(appStates && appStates[this.appName]);
    this.service.updateCharacteristic(this.platform.Characteristic.On, running);
  }

  private handleOnGet(): CharacteristicValue {
    const appStates = this.client.appStates;
    return !!(appStates && appStates[this.appName]);
  }

  private async handleOnSet(value: CharacteristicValue): Promise<void> {
    const targetState = value as boolean;
    const target = targetState ? 'on' : 'off';

    // Wake before launch: if computer is already reachable, skip WoL; else WoL → 5s → wake-screen
    const appConfig = this.client.managedApps?.find(
      (a) => a.name.toLowerCase() === this.appName.toLowerCase(),
    );
    if (target === 'on' && appConfig?.wakeBefore && this.client.mac) {
      const reachable = await this.platform.quickReachabilityCheck(
        this.client.ip,
        this.client.port,
        this.client.token,
      );
      if (reachable) {
        this.platform.log.info(`✅ Computer already online, skipping WoL: ${this.client.hostname}`);
      } else {
        this.platform.log.info(`⏰ Wake before launch: ${this.client.hostname}`);
        await this.platform.sendWakeOnLan(this.client.mac);
        await new Promise((r) => setTimeout(r, 5000));
        const wakeScreenOk = await this.platform.sendWakeScreenRequest(
          this.client.ip,
          this.client.port,
          this.client.token,
        );
        if (wakeScreenOk) {
          this.platform.log.info(`✅ Wake-screen sent (display on): ${this.client.hostname}`);
        } else {
          this.platform.log.debug(
            `⚠️  Wake-screen failed for ${this.client.hostname} (may be non-macOS or not yet online)`,
          );
        }
      }
    }

    const ok = await this.platform.sendManageAppRequest(
      this.client.ip,
      this.client.port,
      this.appName,
      target,
      this.client.token,
    );

    if (ok) {
      this.platform.log.info(
        `🔘 Managed app: ${this.appName} → ${target} (${this.client.hostname})`,
      );
    } else {
      this.platform.log.warn(
        `⚠️ Manage-app request may have failed for ${this.appName} on ${this.client.hostname}`,
      );
    }
    // Do NOT update characteristic here — next heartbeat will send real app_states
  }

  public updateClient(client: RegisteredClient): void {
    this.client = client;
    this.accessory.context.client = client;
    this.updateFromAppStates(client.appStates);
  }
}

export class ComputerAccessory {
  private service: Service;
  private client: RegisteredClient;
  private isOnline = false;
  private temperatureService: Service | null = null;

  constructor(
    private readonly platform: ComputerControlPlatform,
    private readonly accessory: PlatformAccessory,
  ) {
    this.client = this.accessory.context.client as RegisteredClient;

    // Set accessory information (Name = display name shown in Home app)
    const displayName = this.client.displayName || this.client.hostname || 'Computer';
    this.accessory
      .getService(this.platform.Service.AccessoryInformation)!
      .setCharacteristic(this.platform.Characteristic.Name, displayName)
      .setCharacteristic(this.platform.Characteristic.Manufacturer, 'HomeBridge Computer Control')
      .setCharacteristic(this.platform.Characteristic.Model, this.client.os || 'Unknown OS')
      .setCharacteristic(this.platform.Characteristic.SerialNumber, this.client.mac || 'Unknown');

    // Get or create the Switch service
    this.service =
      this.accessory.getService(this.platform.Service.Switch) ||
      this.accessory.addService(this.platform.Service.Switch);

    // Set display name
    this.service.setCharacteristic(
      this.platform.Characteristic.Name,
      this.client.displayName || this.client.hostname || 'Computer',
    );

    // Register handlers for the On characteristic
    this.service
      .getCharacteristic(this.platform.Characteristic.On)
      .onGet(this.handleOnGet.bind(this))
      .onSet(this.handleOnSet.bind(this));

    // Add TemperatureSensor if client sends temperature data
    this.updateTemperatureService(this.client.temperature);

    // Initial status check
    this.checkOnlineStatus();
  }

  /**
   * Set the accessory to offline (e.g. when client sends "going to sleep").
   */
  public setOffline(): void {
    this.isOnline = false;
    this.service.updateCharacteristic(this.platform.Characteristic.On, false);
  }

  /**
   * Update the client info (called when a new heartbeat arrives).
   * @param setOnline When false (e.g. isDarkWake), do not set device to ONLINE.
   */
  public updateClient(client: RegisteredClient, setOnline = true): void {
    this.client = client;
    this.accessory.context.client = client;

    const displayName = client.displayName || client.hostname || 'Computer';
    // Update Switch service name
    this.service.setCharacteristic(this.platform.Characteristic.Name, displayName);
    // Update accessory and AccessoryInformation so Home app reflects name change
    this.accessory.updateDisplayName(displayName);
    const infoService = this.accessory.getService(this.platform.Service.AccessoryInformation);
    if (infoService) {
      infoService.updateCharacteristic(this.platform.Characteristic.Name, displayName);
    }

    // Add/remove/update TemperatureSensor based on heartbeat data
    this.updateTemperatureService(client.temperature);

    // Only set ONLINE if not Dark Wake (Power Nap)
    if (setOnline) {
      this.isOnline = true;
      this.service.updateCharacteristic(this.platform.Characteristic.On, true);
    }
  }

  /**
   * Add, remove, or update the TemperatureSensor service based on client temperature data.
   * Millidegree → Celsius: divide by 1000.
   */
  private updateTemperatureService(temperatureMillidegree?: number | null): void {
    const hasTemperature = typeof temperatureMillidegree === 'number' && temperatureMillidegree > 0;
    const celsius = hasTemperature ? temperatureMillidegree / 1000 : 0;
    let changed = false;

    if (hasTemperature) {
      if (!this.temperatureService) {
        this.temperatureService =
          this.accessory.getServiceById(this.platform.Service.TemperatureSensor, TEMPERATURE_SERVICE_SUBTYPE) ||
          this.accessory.addService(
            this.platform.Service.TemperatureSensor,
            'CPU Temperature',
            TEMPERATURE_SERVICE_SUBTYPE,
          );
        this.platform.log.info(`➕ TemperatureSensor added for ${this.client.hostname} (${celsius}°C)`);
        changed = true;
      }
      this.temperatureService.updateCharacteristic(
        this.platform.Characteristic.CurrentTemperature,
        celsius,
      );
    } else {
      if (this.temperatureService) {
        this.accessory.removeService(this.temperatureService);
        this.temperatureService = null;
        this.platform.log.info(`🗑️ TemperatureSensor removed from ${this.client.hostname}`);
        changed = true;
      }
    }

    if (changed) {
      this.platform.api.updatePlatformAccessories([this.accessory]);
    }
  }

  /**
   * Handle HomeKit GET request for the On state.
   */
  private handleOnGet(): CharacteristicValue {
    this.platform.log.debug(
      `📖 GET On -> ${this.isOnline} (${this.client.hostname})`,
    );
    return this.isOnline;
  }

  /**
   * Handle HomeKit SET request for the On state.
   *
   * - true  (ON)  → Send Wake-on-LAN magic packet
   * - false (OFF) → Send sleep request to the client
   */
  private async handleOnSet(value: CharacteristicValue): Promise<void> {
    const targetState = value as boolean;

    if (targetState) {
      // ──── WAKE (Power On) ────
      this.platform.log.info(
        `⏰ Waking up ${this.client.hostname} (${this.client.mac})`,
      );

      try {
        // 1. Send WoL magic packet to wake the system
        await this.platform.sendWakeOnLan(this.client.mac);
        this.platform.log.info(
          `✅ WoL packet sent to ${this.client.mac}`,
        );

        // 2. Wait 2-3 seconds for device to connect to network
        await new Promise((r) => setTimeout(r, 5000)); // 5s for deep sleep wake

        // 3. Send wake-screen request to force display on (macOS: caffeinate)
        const wakeScreenOk = await this.platform.sendWakeScreenRequest(
          this.client.ip,
          this.client.port,
          this.client.token,
        );
        if (wakeScreenOk) {
          this.platform.log.info(
            `✅ Wake-screen sent to ${this.client.hostname}`,
          );
        } else {
          this.platform.log.debug(
            `⚠️  Wake-screen failed or skipped for ${this.client.hostname} (may be non-macOS or not yet online)`,
          );
        }

        // The computer won't be online instantly, but we set it to ON
        // and let the status checker verify later.
        setTimeout(() => this.checkOnlineStatus(), 10_000);
      } catch (err) {
        this.platform.log.error(
          `❌ Failed to send WoL to ${this.client.mac}:`,
          (err as Error).message,
        );
      }
    } else {
      // ──── SLEEP (Power Off) ────
      this.platform.log.info(
        `💤 Putting ${this.client.hostname} to sleep (${this.client.ip}:${this.client.port})`,
      );

      const success = await this.platform.sendSleepRequest(
        this.client.ip,
        this.client.port,
        this.client.token,
      );

      if (success) {
        this.platform.log.info(
          `✅ Sleep command sent to ${this.client.hostname}`,
        );
        this.isOnline = false;
      } else {
        this.platform.log.warn(
          `⚠️  Sleep command may have failed for ${this.client.hostname}`,
        );
        // Even if the request "failed", the machine might have gone to sleep
        // before it could respond. Set to offline anyway.
        this.isOnline = false;
      }
    }
  }

  /**
   * Check if the client app is online. Uses HTTP /health only — no ping fallback.
   * Ping would show machine reachable even when our client app is not running.
   */
  public async checkOnlineStatus(): Promise<void> {
    const macKey = this.client.mac.toUpperCase();

    // State Lock: keep OFF during sleep debounce window
    if (this.platform.isInSleepDebounceWindow(macKey)) {
      if (this.isOnline) {
        this.isOnline = false;
        this.service.updateCharacteristic(this.platform.Characteristic.On, false);
      }
      return;
    }

    try {
      // HTTP only: client app must respond. Ping would falsely show ONLINE when app is closed.
      const alive = await this.platform.httpHealthCheck(this.client.ip, this.client.port, this.client.token);

      const previousState = this.isOnline;
      this.isOnline = alive;

      if (previousState !== alive) {
        this.platform.log.info(
          `${alive ? '🟢' : '🔴'} ${this.client.hostname} is now ${alive ? 'ONLINE' : 'OFFLINE'}`,
        );
      }

      this.service.updateCharacteristic(
        this.platform.Characteristic.On,
        alive,
      );
    } catch (err) {
      this.platform.log.debug(
        `⚠️  Health check failed for ${this.client.hostname}: ${(err as Error).message}`,
      );
      this.isOnline = false;
      this.service.updateCharacteristic(
        this.platform.Characteristic.On,
        false,
      );
    }
  }
}
