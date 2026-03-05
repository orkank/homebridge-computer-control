declare module 'wake_on_lan' {
  interface WakeOptions {
    address?: string;
    num_packets?: number;
    interval?: number;
    port?: number;
  }

  function wake(mac: string, callback: (err: Error | null) => void): void;
  function wake(mac: string, options: WakeOptions, callback: (err: Error | null) => void): void;

  function createMagicPacket(mac: string): Buffer;
}
