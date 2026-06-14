import { invoke } from '@tauri-apps/api/core';
import { open } from '@tauri-apps/plugin-dialog';

export type StatusResponse = {
  ok: boolean;
  status: string;
  detail: string;
};

async function call<T>(command: string): Promise<T> {
  return invoke<T>(command);
}

export const desktop = {
  serverStatus: () => call<StatusResponse>('server_status'),
  providerStatus: () => call<StatusResponse>('provider_status'),
  startServer: () => call<StatusResponse>('start_server'),
  stopServer: () => call<StatusResponse>('stop_server'),
  chooseProjectFolder: async () => {
    const selected = await open({ directory: true, multiple: false, title: 'Choose Zero project folder' });
    return typeof selected === 'string' ? selected : null;
  },
};
