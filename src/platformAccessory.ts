import {
  Service,
  PlatformAccessory,
  CharacteristicValue,
} from 'homebridge';

import { ComputerControlPlatform } from './platform';
import { RegisteredClient } from './settings';

export const GROUP_ACCESSORY_UUID = 'computer-control-group';
export const ANTI_SLEEP_ACCESSORY_UUID = 'computer-control-anti-sleep';

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
 * ComputerAccessory
 *
 * Handles individual computer accessories in HomeKit.
 * Each accessory is a Switch that:
 * - ON  = Wake the computer (Wake-on-LAN)
 * - OFF = Sleep the computer (HTTP request to client)
 *
 * Status is determined by pinging the client.
 */
export class ComputerAccessory {
  private service: Service;
  private client: RegisteredClient;
  private isOnline = false;

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

    // Only set ONLINE if not Dark Wake (Power Nap)
    if (setOnline) {
      this.isOnline = true;
      this.service.updateCharacteristic(this.platform.Characteristic.On, true);
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
