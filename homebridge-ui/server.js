const fs = require('fs');
const path = require('path');
const http = require('http');

(async () => {
  const { HomebridgePluginUiServer } = await import('@homebridge/plugin-ui-utils');

  class PluginUiServer extends HomebridgePluginUiServer {
    constructor() {
      super();

      this.onRequest('/get-clients', this.getClients.bind(this));
      this.onRequest('/delete-client', this.deleteClient.bind(this));
      this.onRequest('/fetch-client-health', this.fetchClientHealth.bind(this));
      this.onRequest('/fetch-client-status', this.fetchClientStatus.bind(this));

      this.ready();
    }

    async getClients() {
      try {
        const storagePath = this.homebridgeStoragePath;
        const clientsFile = path.join(storagePath, 'computer-control-clients.json');

        if (fs.existsSync(clientsFile)) {
          const data = fs.readFileSync(clientsFile, 'utf8');
          const parsed = JSON.parse(data);
          return Object.values(parsed).map((c) => {
            const { token, ...rest } = c;
            return rest;
          });
        }
        return [];
      } catch (err) {
        console.error('Failed to read clients file:', err);
        return [];
      }
    }

    async deleteClient(payload) {
      const { mac, port = 9090 } = payload || {};
      if (!mac) {
        return { success: false, error: 'Missing MAC address' };
      }

      return new Promise((resolve) => {
        const encodedMac = encodeURIComponent(mac);
        const req = http.request(
          {
            hostname: 'localhost',
            port: Number(port),
            path: `/clients/${encodedMac}`,
            method: 'DELETE',
            timeout: 5000,
          },
          (res) => {
            let body = '';
            res.on('data', (chunk) => (body += chunk));
            res.on('end', () => {
              if (res.statusCode === 200) {
                resolve({ success: true });
              } else {
                try {
                  const err = JSON.parse(body);
                  resolve({ success: false, error: err.error || 'Delete failed' });
                } catch {
                  resolve({ success: false, error: `HTTP ${res.statusCode}` });
                }
              }
            });
          },
        );
        req.on('error', (err) => resolve({ success: false, error: err.message }));
        req.on('timeout', () => {
          req.destroy();
          resolve({ success: false, error: 'Request timeout' });
        });
        req.end();
      });
    }

    async fetchClientHealth(payload) {
      const { mac, port = 9090 } = payload || {};
      if (!mac) {
        return { success: false, error: 'Missing MAC address' };
      }
      return this.proxyToClient(mac, 'health', Number(port));
    }

    async fetchClientStatus(payload) {
      const { mac, port = 9090 } = payload || {};
      if (!mac) {
        return { success: false, error: 'Missing MAC address' };
      }
      return this.proxyToClient(mac, 'status', Number(port));
    }

    proxyToClient(mac, endpoint, port) {
      return new Promise((resolve) => {
        const encodedMac = encodeURIComponent(mac);
        const req = http.request(
          {
            hostname: 'localhost',
            port,
            path: `/clients/${encodedMac}/${endpoint}`,
            method: 'GET',
            timeout: 15000,
          },
          (res) => {
            let body = '';
            res.on('data', (chunk) => (body += chunk));
            res.on('end', () => {
              if (res.statusCode === 200) {
                try {
                  const data = body ? JSON.parse(body) : {};
                  resolve({ success: true, data });
                } catch {
                  resolve({ success: true, data: { raw: body } });
                }
              } else {
                try {
                  const err = JSON.parse(body);
                  resolve({ success: false, error: err.error || err.details || 'Request failed' });
                } catch {
                  resolve({ success: false, error: `HTTP ${res.statusCode}` });
                }
              }
            });
          },
        );
        req.on('error', (err) => resolve({ success: false, error: err.message }));
        req.on('timeout', () => {
          req.destroy();
          resolve({ success: false, error: 'Request timeout' });
        });
        req.end();
      });
    }
  }

  new PluginUiServer();
})();
