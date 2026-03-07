import {
  API,
  DynamicPlatformPlugin,
  Logger,
  PlatformAccessory,
  PlatformConfig,
  Service,
  Characteristic,
} from 'homebridge';

import * as crypto from 'crypto';
import * as http from 'http';
import * as wol from 'wake_on_lan';
import * as fs from 'fs';
import * as path from 'path';

import {
  PLATFORM_NAME,
  PLUGIN_NAME,
  REGISTRATION_PORT,
  DEFAULT_CLIENT_PORT,
  CLIENT_VERSION,
  STATUS_CHECK_INTERVAL,
  PING_TIMEOUT,
  SLEEP_DEBOUNCE_SECONDS,
  RegisteredClient,
} from './settings';

import { ComputerAccessory, GroupComputerAccessory, AntiSleepAccessory, GROUP_ACCESSORY_UUID, ANTI_SLEEP_ACCESSORY_UUID } from './platformAccessory';

/**
 * Maps URL path segments to actual binary file names in the /bin directory.
 */
const DOWNLOAD_MAP: Record<string, string> = {
  'darwin-arm64': 'computer-control-darwin-arm64',
  'darwin-amd64': 'computer-control-darwin-amd64',
  'windows-amd64': 'computer-control-windows-amd64.exe',
  'windows-arm64': 'computer-control-windows-arm64.exe',
  'linux-amd64': 'computer-control-linux-amd64',
  'linux-arm64': 'computer-control-linux-arm64',
  'darwin-app': 'computer-control-darwin-app.zip',
};

/** Map client os+arch to download platform key. Prefer darwin-app for macOS. */
function getPlatformKey(os: string, arch?: string): string {
  const o = (os || '').toLowerCase();
  const a = (arch || 'amd64').toLowerCase();
  if (o === 'darwin') return 'darwin-app';
  if (o === 'windows') return a === 'arm64' ? 'windows-arm64' : 'windows-amd64';
  if (o === 'linux') return a === 'arm64' ? 'linux-arm64' : 'linux-amd64';
  return 'darwin-app'; // fallback
}

/** Simple semver compare: returns true if a < b */
function semverLt(a: string, b: string): boolean {
  const pa = a.split('.').map((n) => parseInt(n, 10) || 0);
  const pb = b.split('.').map((n) => parseInt(n, 10) || 0);
  for (let i = 0; i < Math.max(pa.length, pb.length); i++) {
    const va = pa[i] ?? 0;
    const vb = pb[i] ?? 0;
    if (va < vb) return true;
    if (va > vb) return false;
  }
  return false;
}

/** SHA256 cache for bin files (platform -> hash) */
let sha256Cache: Map<string, string> | null = null;

function getSha256ForPlatform(platform: string): string | null {
  if (!sha256Cache) {
    sha256Cache = new Map();
  }
  if (sha256Cache.has(platform)) {
    return sha256Cache.get(platform)!;
  }
  const filename = DOWNLOAD_MAP[platform];
  if (!filename) return null;
  const binDir = path.join(__dirname, '..', 'bin');
  const filePath = path.join(binDir, filename);
  if (!fs.existsSync(filePath)) return null;

  const buf = fs.readFileSync(filePath);
  const hash = crypto.createHash('sha256').update(buf).digest('hex');
  sha256Cache.set(platform, hash);
  return hash;
}

/**
 * ComputerControlPlatform
 *
 * Main platform class that:
 * - Runs an HTTP server to receive client registrations
 * - Manages registered clients in a local JSON file
 * - Creates/removes HomeKit accessories dynamically
 * - Checks client status periodically via ping
 * - Serves pre-compiled client binaries for download
 */
export class ComputerControlPlatform implements DynamicPlatformPlugin {
  public readonly Service: typeof Service = this.api.hap.Service;
  public readonly Characteristic: typeof Characteristic = this.api.hap.Characteristic;

  // Cached accessories from Homebridge
  public readonly accessories: PlatformAccessory[] = [];

  // Registered clients (in-memory)
  private clients: Map<string, RegisteredClient> = new Map();

  // Path to the persisted clients file
  private readonly clientsFilePath: string;

  // Path to the bin directory containing pre-compiled clients
  private readonly binDir: string;

  // Registration HTTP server
  private registrationServer: http.Server | null = null;

  // Managed accessory handlers
  private readonly computerAccessories: Map<string, ComputerAccessory> = new Map();
  private groupAccessory: GroupComputerAccessory | null = null;
  private antiSleepAccessory: AntiSleepAccessory | null = null;

  // MAC -> timestamp when we received "going to sleep". Used for 10s debounce.
  private readonly sleepTimestamps: Map<string, number> = new Map();

  // MAC -> already logged "update available" for this client (avoid log spam)
  private readonly updateLoggedFor: Set<string> = new Set();

  constructor(
    public readonly log: Logger,
    public readonly config: PlatformConfig,
    public readonly api: API,
  ) {
    this.log.info('🖥️  ComputerControl platform initializing...');

    // Set up paths
    this.clientsFilePath = path.join(api.user.storagePath(), 'computer-control-clients.json');

    // bin/ is at the root of the plugin package
    this.binDir = path.join(__dirname, '..', 'bin');

    // Load previously registered clients
    this.loadClients();

    // When Homebridge has finished launching, start our services
    this.api.on('didFinishLaunching', () => {
      this.log.info('✅ Homebridge finished launching');
      this.startRegistrationServer();
      this.discoverDevices();
      this.startStatusChecker();
    });
  }

  // ──────────────────────────────────────────────
  // Homebridge Lifecycle
  // ──────────────────────────────────────────────

  /**
   * Called by Homebridge to restore cached accessories from disk.
   */
  configureAccessory(accessory: PlatformAccessory): void {
    this.log.info('📦 Restoring cached accessory: %s', accessory.displayName);
    this.accessories.push(accessory);
  }

  /**
   * Returns all registered clients (for group accessory).
   */
  public getClients(): RegisteredClient[] {
    return Array.from(this.clients.values());
  }

  // ──────────────────────────────────────────────
  // Client Registration & API Server
  // ──────────────────────────────────────────────

  /**
   * Start the HTTP server that listens for client registrations,
   * serves the client list, and provides binary downloads.
   */
  private startRegistrationServer(): void {
    const port = (this.config.registrationPort as number) || REGISTRATION_PORT;

    this.registrationServer = http.createServer((req, res) => {
      // CORS headers for convenience
      res.setHeader('Access-Control-Allow-Origin', '*');
      res.setHeader('Access-Control-Allow-Methods', 'GET, POST, DELETE, OPTIONS');
      res.setHeader('Access-Control-Allow-Headers', 'Content-Type');

      if (req.method === 'OPTIONS') {
        res.writeHead(200);
        res.end();
        return;
      }

      const url = req.url || '';

      // ── Registration ──
      if (req.method === 'POST' && url === '/register') {
        this.handleRegistration(req, res);
        return;
      }

      // ── Going to Sleep (client notifies before sleeping) ──
      if (req.method === 'POST' && url === '/going-to-sleep') {
        this.handleGoingToSleep(req, res);
        return;
      }

      // ── Client List ──
      if (req.method === 'GET' && url === '/clients') {
        this.handleListClients(res);
        return;
      }

      // ── Delete Client ──
      if (req.method === 'DELETE' && url.startsWith('/clients/')) {
        this.handleDeleteClient(req, res);
        return;
      }

      // ── Proxy: Fetch health/status from client (for Config UI; plugin has token) ──
      const clientProxyMatch = url.match(/^\/clients\/([^/]+)\/(health|status)$/);
      if (req.method === 'GET' && clientProxyMatch) {
        const mac = decodeURIComponent(clientProxyMatch[1]).toUpperCase();
        const endpoint = clientProxyMatch[2] as 'health' | 'status';
        this.handleClientProxy(mac, endpoint, res);
        return;
      }

      // ── Download: List available binaries ──
      if (req.method === 'GET' && url === '/download') {
        this.handleDownloadList(res);
        return;
      }

      // ── Download: Serve a specific binary ──
      if (req.method === 'GET' && url.startsWith('/download/')) {
        this.handleDownloadBinary(req, res);
        return;
      }

      // ── 404 ──
      res.writeHead(404, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({ error: 'Not found' }));
    });

    this.registrationServer.listen(port, () => {
      this.log.info(`🌐 Registration server listening on port ${port}`);
      this.log.info(`📥 Client downloads available at http://localhost:${port}/download`);
    });

    this.registrationServer.on('error', (err) => {
      this.log.error('❌ Registration server error:', err.message);
    });
  }

  /**
   * Handle "going to sleep" notification from client.
   * Client sends this before sleeping so we can immediately set device to OFF.
   */
  private handleGoingToSleep(req: http.IncomingMessage, res: http.ServerResponse): void {
    let body = '';
    req.on('data', (chunk) => (body += chunk));
    req.on('end', () => {
      try {
        const data = body ? (JSON.parse(body) as { mac?: string }) : {};
        const mac = data.mac;
        if (!mac) {
          res.writeHead(400, { 'Content-Type': 'application/json' });
          res.end(JSON.stringify({ error: 'Missing mac' }));
          return;
        }
        const macKey = mac.toUpperCase();
        this.sleepTimestamps.set(macKey, Date.now());

        const handler = this.computerAccessories.get(macKey);
        if (handler) {
          handler.setOffline();
          this.log.info(`💤 ${macKey} going to sleep — set OFF immediately`);
        }

        res.writeHead(200, { 'Content-Type': 'application/json' });
        res.end(JSON.stringify({ success: true, message: 'Marked offline' }));
      } catch (err) {
        this.log.error('❌ Failed to parse going-to-sleep:', (err as Error).message);
        res.writeHead(400, { 'Content-Type': 'application/json' });
        res.end(JSON.stringify({ error: 'Invalid JSON' }));
      }
    });
  }

  /**
   * Handle incoming client registration (heartbeat).
   */
  private handleRegistration(req: http.IncomingMessage, res: http.ServerResponse): void {
    let body = '';
    req.on('data', (chunk) => (body += chunk));
    req.on('end', () => {
      try {
        const data = JSON.parse(body) as RegisteredClient & { version?: string; arch?: string };

        if (!data.mac || !data.ip) {
          res.writeHead(400, { 'Content-Type': 'application/json' });
          res.end(JSON.stringify({ error: 'Missing required fields: mac, ip' }));
          return;
        }

        // Normalize MAC address as key
        const macKey = data.mac.toUpperCase();

        // State Lock: ignore all signals for 20s after "going to sleep"
        const sleepAt = this.sleepTimestamps.get(macKey);
        if (sleepAt && Date.now() - sleepAt < SLEEP_DEBOUNCE_SECONDS * 1000) {
          this.log.debug(`⏭️  Ignoring registration from ${macKey} (within ${SLEEP_DEBOUNCE_SECONDS}s of sleep)`);
          res.writeHead(200, { 'Content-Type': 'application/json' });
          res.end(JSON.stringify({ success: true, message: 'Ignored (debounce)' }));
          return;
        }

        const existing = this.clients.get(macKey);
        const isDarkWake = !!data.isDarkWake;

        // Use existing token or generate new one (client must send token in all requests)
        const token = existing?.token || crypto.randomBytes(32).toString('hex');

        const temperature = typeof data.temperature === 'number' && data.temperature > 0
          ? data.temperature
          : undefined;

        this.log.info(
          `📥 Registration payload: hostname=${data.hostname}, mac=${macKey}, temperature=${temperature ?? 'none'}${temperature ? ` (${temperature / 1000}°C)` : ''}`,
        );

        const client: RegisteredClient = {
          hostname: data.hostname || 'Unknown',
          ip: data.ip,
          mac: data.mac,
          port: data.port || DEFAULT_CLIENT_PORT,
          os: data.os || 'unknown',
          lastSeen: Date.now(),
          displayName: data.hostname || existing?.displayName || 'Computer',
          isDarkWake,
          token,
          temperature,
        };

        this.clients.set(macKey, client);
        this.saveClients();

        // Register or update the accessory (isDarkWake: do not set ONLINE; temperature: add/remove sensor)
        this.addOrUpdateAccessory(macKey, client, isDarkWake);

        this.log.info(
          `💓 Registration from ${client.hostname} (${client.ip} / ${client.mac})${isDarkWake ? ' [Dark Wake — kept OFF]' : ''}`,
        );

        const response: Record<string, unknown> = { success: true, message: 'Registered', token };

        // Auto-update: if client version is older, include update info
        const clientVersion = data.version || '';
        if (clientVersion && clientVersion !== 'dev' && semverLt(clientVersion, CLIENT_VERSION)) {
          const platform = getPlatformKey(data.os || 'unknown', data.arch);
          const sha256 = getSha256ForPlatform(platform);
          const host = req.headers.host || `localhost:${(this.config.registrationPort as number) || REGISTRATION_PORT}`;
          const protocol = req.headers['x-forwarded-proto'] === 'https' ? 'https' : 'http';
          const updateUrl = `${protocol}://${host}/download/${platform}`;

          if (sha256) {
            response.updateAvailable = true;
            response.updateUrl = updateUrl;
            response.updateSha256 = sha256;
            if (!this.updateLoggedFor.has(macKey)) {
              this.updateLoggedFor.add(macKey);
              this.log.info(`📦 Update available for ${client.hostname}: ${clientVersion} → ${CLIENT_VERSION}`);
            }
          }
        }

        res.writeHead(200, { 'Content-Type': 'application/json' });
        res.end(JSON.stringify(response));
      } catch (err) {
        this.log.error('❌ Failed to parse registration:', (err as Error).message);
        res.writeHead(400, { 'Content-Type': 'application/json' });
        res.end(JSON.stringify({ error: 'Invalid JSON' }));
      }
    });
  }

  /**
   * Return the list of registered clients.
   */
  private handleListClients(res: http.ServerResponse): void {
    const clientList = Array.from(this.clients.values());
    res.writeHead(200, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify(clientList, null, 2));
  }

  /**
   * Delete a registered client by MAC address.
   */
  private handleDeleteClient(req: http.IncomingMessage, res: http.ServerResponse): void {
    const mac = decodeURIComponent(req.url!.replace('/clients/', '')).toUpperCase();

    if (!this.clients.has(mac)) {
      res.writeHead(404, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({ error: 'Client not found' }));
      return;
    }

    this.clients.delete(mac);
    this.updateLoggedFor.delete(mac);
    this.saveClients();

    // Remove the accessory from HomeKit
    this.removeAccessory(mac);

    this.log.info(`🗑️  Client removed: ${mac}`);

    res.writeHead(200, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify({ success: true, message: 'Deleted' }));
  }

  /**
   * Proxy health or status request to a client. Used by Config UI so users can fetch
   * client info without needing the token (plugin has it).
   */
  private handleClientProxy(macKey: string, endpoint: 'health' | 'status', res: http.ServerResponse): void {
    const client = this.clients.get(macKey);
    if (!client) {
      res.writeHead(404, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({ error: 'Client not found' }));
      return;
    }

    const path = `/${endpoint}`;
    const headers: Record<string, string> = { 'Content-Type': 'application/json' };
    if (client.token) headers['X-Auth-Token'] = client.token;

    const proxyReq = http.request(
      {
        hostname: client.ip,
        port: client.port,
        path,
        method: 'GET',
        timeout: PING_TIMEOUT * 1000,
        headers,
      },
      (proxyRes) => {
        let body = '';
        proxyRes.on('data', (chunk) => (body += chunk));
        proxyRes.on('end', () => {
          res.writeHead(proxyRes.statusCode || 500, { 'Content-Type': 'application/json' });
          if (body) {
            res.end(body);
          } else {
            res.end(JSON.stringify({ error: 'Empty response', statusCode: proxyRes.statusCode }));
          }
        });
      },
    );

    proxyReq.on('error', (err) => {
      res.writeHead(502, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({ error: err.message, details: 'Could not reach client' }));
    });
    proxyReq.on('timeout', () => {
      proxyReq.destroy();
      res.writeHead(504, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({ error: 'Request timeout', details: 'Client did not respond in time' }));
    });
    proxyReq.end();
  }

  // ──────────────────────────────────────────────
  // Binary Download Endpoints
  // ──────────────────────────────────────────────

  /**
   * List available client binaries for download.
   * GET /download
   */
  private handleDownloadList(res: http.ServerResponse): void {
    const available: { platform: string; filename: string; url: string; exists: boolean }[] = [];

    for (const [key, filename] of Object.entries(DOWNLOAD_MAP)) {
      const filePath = path.join(this.binDir, filename);
      const exists = fs.existsSync(filePath);
      available.push({
        platform: key,
        filename,
        url: `/download/${key}`,
        exists,
      });
    }

    res.writeHead(200, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify({
      message: 'Available client binaries. Use GET /download/<platform> to download.',
      binaries: available,
    }, null, 2));
  }

  /**
   * Serve a pre-compiled binary for download.
   * GET /download/:platform  (e.g. /download/darwin-arm64)
   */
  private handleDownloadBinary(req: http.IncomingMessage, res: http.ServerResponse): void {
    const platform = req.url!.replace('/download/', '').toLowerCase();
    const filename = DOWNLOAD_MAP[platform];

    if (!filename) {
      res.writeHead(400, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({
        error: `Unknown platform: ${platform}`,
        available: Object.keys(DOWNLOAD_MAP),
      }));
      return;
    }

    const filePath = path.join(this.binDir, filename);

    if (!fs.existsSync(filePath)) {
      res.writeHead(404, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({
        error: `Binary not found: ${filename}. Run 'npm run build:clients' to compile.`,
      }));
      return;
    }

    const stat = fs.statSync(filePath);
    const downloadName = platform === 'darwin-app' ? 'ComputerControl.app.zip' : filename;

    res.writeHead(200, {
      'Content-Type': platform === 'darwin-app' ? 'application/zip' : 'application/octet-stream',
      'Content-Disposition': `attachment; filename="${downloadName}"`,
      'Content-Length': stat.size,
    });

    const readStream = fs.createReadStream(filePath);
    readStream.pipe(res);

    this.log.info(`📥 Client binary downloaded: ${filename}`);
  }

  // ──────────────────────────────────────────────
  // Accessory Management
  // ──────────────────────────────────────────────

  /**
   * Discover devices from the saved clients file and register accessories.
   */
  private discoverDevices(): void {
    this.log.info(`🔍 Discovering devices... (${this.clients.size} registered)`);

    for (const [macKey, client] of this.clients) {
      this.addOrUpdateAccessory(macKey, client);
    }

    // Add or update the virtual group accessory (Wake All / Sleep All)
    this.addOrUpdateGroupAccessory();

    // Add or update the Anti-Sleep accessory (if configured)
    this.addOrUpdateAntiSleepAccessory();

    // Remove any client accessories that no longer have a corresponding client
    // (keep the group accessory)
    const clientUUIDs = new Set(
      Array.from(this.clients.keys()).map((mac) =>
        this.api.hap.uuid.generate(mac),
      ),
    );
    const groupUUID = this.api.hap.uuid.generate(GROUP_ACCESSORY_UUID);
    const antiSleepUUID = this.api.hap.uuid.generate(ANTI_SLEEP_ACCESSORY_UUID);

    const accessoriesToRemove = this.accessories.filter(
      (acc) =>
        acc.UUID !== groupUUID &&
        acc.UUID !== antiSleepUUID &&
        !clientUUIDs.has(acc.UUID),
    );

    if (accessoriesToRemove.length > 0) {
      this.log.info(`🗑️  Removing ${accessoriesToRemove.length} stale accessories`);
      this.api.unregisterPlatformAccessories(PLUGIN_NAME, PLATFORM_NAME, accessoriesToRemove);
    }
  }

  /**
   * Add or update the virtual group accessory (controls all computers).
   */
  private addOrUpdateGroupAccessory(): void {
    const displayName = (this.config.groupAccessoryName as string) || 'Computers';
    const uuid = this.api.hap.uuid.generate(GROUP_ACCESSORY_UUID);
    const existing = this.accessories.find((acc) => acc.UUID === uuid);

    if (existing) {
      existing.updateDisplayName(displayName);
      const infoService = existing.getService(this.Service.AccessoryInformation);
      if (infoService) {
        infoService.updateCharacteristic(this.Characteristic.Name, displayName);
      }
      this.api.updatePlatformAccessories([existing]);

      if (this.groupAccessory) {
        this.groupAccessory.updateDisplayName(displayName);
      } else {
        this.groupAccessory = new GroupComputerAccessory(this, existing, displayName);
      }
      this.log.debug(`🔄 Updated group accessory: ${displayName}`);
    } else {
      const accessory = new this.api.platformAccessory(displayName, uuid);
      accessory.context.isGroup = true;

      this.groupAccessory = new GroupComputerAccessory(this, accessory, displayName);
      this.api.registerPlatformAccessories(PLUGIN_NAME, PLATFORM_NAME, [accessory]);
      this.accessories.push(accessory);
      this.log.info(`➕ Added group accessory: ${displayName}`);
    }
  }

  /**
   * Add or update the Anti-Sleep accessory (prevents all computers from sleeping).
   * Only created if antiSleepDeviceName is non-empty.
   */
  private addOrUpdateAntiSleepAccessory(): void {
    const displayName = (this.config.antiSleepDeviceName as string)?.trim();
    const uuid = this.api.hap.uuid.generate(ANTI_SLEEP_ACCESSORY_UUID);
    const existing = this.accessories.find((acc) => acc.UUID === uuid);

    if (!displayName) {
      if (existing) {
        this.api.unregisterPlatformAccessories(PLUGIN_NAME, PLATFORM_NAME, [existing]);
        this.accessories.splice(this.accessories.indexOf(existing), 1);
        this.antiSleepAccessory = null;
        this.log.info('🗑️  Anti-Sleep accessory removed (name empty)');
      }
      return;
    }

    if (existing) {
      existing.updateDisplayName(displayName);
      const infoService = existing.getService(this.Service.AccessoryInformation);
      if (infoService) {
        infoService.updateCharacteristic(this.Characteristic.Name, displayName);
      }
      this.api.updatePlatformAccessories([existing]);

      if (this.antiSleepAccessory) {
        this.antiSleepAccessory.updateDisplayName(displayName);
      } else {
        this.antiSleepAccessory = new AntiSleepAccessory(this, existing, displayName);
      }
      this.log.debug(`🔄 Updated Anti-Sleep accessory: ${displayName}`);
    } else {
      const accessory = new this.api.platformAccessory(displayName, uuid);
      accessory.context.isAntiSleep = true;

      this.antiSleepAccessory = new AntiSleepAccessory(this, accessory, displayName);
      this.api.registerPlatformAccessories(PLUGIN_NAME, PLATFORM_NAME, [accessory]);
      this.accessories.push(accessory);
      this.log.info(`➕ Added Anti-Sleep accessory: ${displayName}`);
    }
  }

  /**
   * Add a new accessory or update an existing one.
   * @param isDarkWake When true (macOS Power Nap), do not set device to ONLINE.
   */
  private addOrUpdateAccessory(macKey: string, client: RegisteredClient, isDarkWake = false): void {
    const uuid = this.api.hap.uuid.generate(macKey);
    const displayName = client.displayName || client.hostname || 'Computer';

    // Check if accessory already exists
    const existingAccessory = this.accessories.find((acc) => acc.UUID === uuid);

    if (existingAccessory) {
      // Update context, display name, and re-initialize handler
      existingAccessory.context.client = client;
      existingAccessory.updateDisplayName(displayName);
      // Update AccessoryInformation Name so Home app reflects the change
      const infoService = existingAccessory.getService(this.Service.AccessoryInformation);
      if (infoService) {
        infoService.updateCharacteristic(this.Characteristic.Name, displayName);
      }
      this.api.updatePlatformAccessories([existingAccessory]);

      // Update or create handler (isDarkWake: don't set ONLINE)
      if (!this.computerAccessories.has(macKey)) {
        this.computerAccessories.set(
          macKey,
          new ComputerAccessory(this, existingAccessory),
        );
      } else {
        this.computerAccessories.get(macKey)!.updateClient(client, !isDarkWake);
      }

      this.log.debug(`🔄 Updated accessory: ${displayName}`);
    } else {
      // Create new accessory
      const accessory = new this.api.platformAccessory(displayName, uuid);
      accessory.context.client = client;

      const handler = new ComputerAccessory(this, accessory);
      this.computerAccessories.set(macKey, handler);

      this.api.registerPlatformAccessories(PLUGIN_NAME, PLATFORM_NAME, [accessory]);
      this.accessories.push(accessory);

      this.log.info(`➕ Added new accessory: ${displayName}`);
    }
  }

  /**
   * Remove an accessory by MAC key.
   */
  private removeAccessory(macKey: string): void {
    const uuid = this.api.hap.uuid.generate(macKey);
    const index = this.accessories.findIndex((acc) => acc.UUID === uuid);

    if (index !== -1) {
      const accessory = this.accessories[index];
      this.api.unregisterPlatformAccessories(PLUGIN_NAME, PLATFORM_NAME, [accessory]);
      this.accessories.splice(index, 1);
      this.computerAccessories.delete(macKey);
      this.log.info(`🗑️  Removed accessory: ${accessory.displayName}`);
    }
  }

  // ──────────────────────────────────────────────
  // Status Checker (Ping)
  // ──────────────────────────────────────────────

  /**
   * Periodically check if each registered client is online.
   */
  private startStatusChecker(): void {
    setInterval(() => {
      for (const [macKey] of this.clients) {
        const handler = this.computerAccessories.get(macKey);
        if (handler) {
          handler.checkOnlineStatus();
        }
      }
      if (this.groupAccessory) {
        this.groupAccessory.checkOnlineStatus();
      }
    }, STATUS_CHECK_INTERVAL);
  }

  // ──────────────────────────────────────────────
  // Persistence
  // ──────────────────────────────────────────────

  /**
   * Load clients from the JSON file.
   */
  private loadClients(): void {
    try {
      if (fs.existsSync(this.clientsFilePath)) {
        const data = fs.readFileSync(this.clientsFilePath, 'utf8');
        const parsed = JSON.parse(data) as Record<string, RegisteredClient>;
        let needsSave = false;
        for (const [key, client] of Object.entries(parsed)) {
          if (!client.token) {
            client.token = crypto.randomBytes(32).toString('hex');
            parsed[key] = client;
            needsSave = true;
          }
        }
        this.clients = new Map(Object.entries(parsed));
        if (needsSave) {
          this.saveClients();
        }
        this.log.info(`📂 Loaded ${this.clients.size} clients from storage`);
      }
    } catch (err) {
      this.log.error('❌ Failed to load clients file:', (err as Error).message);
    }
  }

  /**
   * Save clients to the JSON file.
   */
  private saveClients(): void {
    try {
      const data: Record<string, RegisteredClient> = {};
      for (const [key, value] of this.clients) {
        data[key] = value;
      }
      fs.writeFileSync(this.clientsFilePath, JSON.stringify(data, null, 2), 'utf8');
    } catch (err) {
      this.log.error('❌ Failed to save clients file:', (err as Error).message);
    }
  }

  // ──────────────────────────────────────────────
  // Public Utilities (used by accessories)
  // ──────────────────────────────────────────────

  /**
   * Send a Wake-on-LAN magic packet to the given MAC address.
   */
  public sendWakeOnLan(mac: string): Promise<void> {
    return new Promise((resolve, reject) => {
      wol.wake(mac, (err: Error | null) => {
        if (err) {
          reject(err);
        } else {
          resolve();
        }
      });
    });
  }

  /**
   * Send a wake-screen request to a client (macOS: caffeinate -u -t 2).
   * Called after WoL to force the display to turn on.
   */
  public sendWakeScreenRequest(ip: string, port: number, token?: string): Promise<boolean> {
    return new Promise((resolve) => {
      const headers: Record<string, string> = {};
      if (token) headers['X-Auth-Token'] = token;

      const req = http.request(
        {
          hostname: ip,
          port,
          path: '/wake-screen',
          method: 'POST',
          timeout: PING_TIMEOUT * 1000,
          headers,
        },
        (res) => {
          let body = '';
          res.on('data', (chunk) => (body += chunk));
          res.on('end', () => {
            try {
              const data = JSON.parse(body);
              resolve(data.success === true);
            } catch {
              resolve(false);
            }
          });
        },
      );

      req.on('error', () => resolve(false));
      req.on('timeout', () => {
        req.destroy();
        resolve(false);
      });

      req.end();
    });
  }

  /**
   * Send a sleep request to a client.
   */
  public sendSleepRequest(ip: string, port: number, token?: string): Promise<boolean> {
    return new Promise((resolve) => {
      const headers: Record<string, string> = {};
      if (token) headers['X-Auth-Token'] = token;

      const req = http.request(
        {
          hostname: ip,
          port,
          path: '/sleep',
          method: 'POST',
          timeout: 10000, // 10s - Windows/firewall may need longer
          headers,
        },
        (res) => {
          let body = '';
          res.on('data', (chunk) => (body += chunk));
          res.on('end', () => {
            try {
              const data = JSON.parse(body);
              resolve(data.success === true);
            } catch {
              resolve(false);
            }
          });
        },
      );

      req.on('error', () => resolve(false));
      req.on('timeout', () => {
        req.destroy();
        resolve(false);
      });

      req.end();
    });
  }

  /**
   * Send stay-awake (enabled/disabled) to all registered clients.
   */
  public async sendStayAwakeToAllClients(enabled: boolean): Promise<void> {
    const clients = this.getClients();
    for (const client of clients) {
      try {
        const ok = await this.sendStayAwakeRequest(client.ip, client.port, client.token, enabled);
        if (ok) {
          this.log.debug(`✅ Stay-awake ${enabled ? 'ON' : 'OFF'} sent to ${client.hostname}`);
        } else {
          this.log.warn(`⚠️ Stay-awake may have failed for ${client.hostname}`);
        }
      } catch (err) {
        this.log.warn(`⚠️ Stay-awake error for ${client.hostname}: ${(err as Error).message}`);
      }
    }
  }

  /**
   * Get anti-sleep timer in minutes (0 = unlimited).
   */
  public getAntiSleepTimer(): number {
    const v = this.config.antiSleepTimer as number | undefined;
    return typeof v === 'number' && v >= 0 ? Math.min(v, 1440) : 0;
  }

  /**
   * Send stay-awake request to a single client.
   */
  public sendStayAwakeRequest(
    ip: string,
    port: number,
    token?: string,
    enabled = true,
  ): Promise<boolean> {
    return new Promise((resolve) => {
      const headers: Record<string, string> = {};
      if (token) headers['X-Auth-Token'] = token;

      const req = http.request(
        {
          hostname: ip,
          port,
          path: `/stay-awake?enabled=${enabled}`,
          method: 'GET',
          timeout: PING_TIMEOUT * 1000,
          headers,
        },
        (res) => {
          let body = '';
          res.on('data', (chunk) => (body += chunk));
          res.on('end', () => {
            try {
              const data = JSON.parse(body);
              resolve(data.success === true);
            } catch {
              resolve(res.statusCode === 200);
            }
          });
        },
      );

      req.on('error', () => resolve(false));
      req.on('timeout', () => {
        req.destroy();
        resolve(false);
      });

      req.end();
    });
  }

  /**
   * Check if a device is within the sleep debounce window (recently went to sleep).
   * During this period we should not trust ping/health or registration.
   */
  public isInSleepDebounceWindow(macKey: string): boolean {
    const sleepAt = this.sleepTimestamps.get(macKey);
    return !!(sleepAt && Date.now() - sleepAt < SLEEP_DEBOUNCE_SECONDS * 1000);
  }

  /**
   * HTTP health check. Parses response: if client reports isDarkWake, treat as OFFLINE.
   */
  public httpHealthCheck(ip: string, port: number, token?: string): Promise<boolean> {
    return new Promise((resolve) => {
      const headers: Record<string, string> = {};
      if (token) headers['X-Auth-Token'] = token;

      const req = http.request(
        {
          hostname: ip,
          port,
          path: '/health',
          method: 'GET',
          timeout: PING_TIMEOUT * 1000,
          headers,
        },
        (res) => {
          let body = '';
          res.on('data', (chunk) => (body += chunk));
          res.on('end', () => {
            if (res.statusCode !== 200) {
              resolve(false);
              return;
            }
            try {
              const data = JSON.parse(body) as {
                isDarkWake?: boolean;
                displayState?: { isDarkWake?: boolean; currentPowerState?: number };
              };
              const isDarkWake = data.isDarkWake ?? data.displayState?.isDarkWake ?? false;
              if (isDarkWake) {
                this.log.debug(
                  `Health: ${ip}:${port} reports Dark Wake (CurrentPowerState=${data.displayState?.currentPowerState ?? '?'}) → OFFLINE`,
                );
              }
              resolve(!isDarkWake);
            } catch {
              resolve(true); // Old client without displayState: treat as online
            }
          });
        },
      );
      req.on('error', () => resolve(false));
      req.on('timeout', () => {
        req.destroy();
        resolve(false);
      });
      req.end();
    });
  }

  /**
   * Ping a client to check if it's online (fallback when HTTP fails).
   */
  public pingClient(ip: string): Promise<boolean> {
    return new Promise((resolve) => {
      const pingModule = require('ping');
      pingModule.sys.probe(ip, (isAlive: boolean) => {
        resolve(isAlive);
      }, { timeout: PING_TIMEOUT });
    });
  }
}
