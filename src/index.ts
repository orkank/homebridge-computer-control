import { API } from 'homebridge';

import { PLATFORM_NAME, PLUGIN_NAME } from './settings';
import { ComputerControlPlatform } from './platform';

/**
 * Plugin entry point.
 * Called by Homebridge to register the platform.
 */
export default (api: API) => {
  api.registerPlatform(PLUGIN_NAME, PLATFORM_NAME, ComputerControlPlatform);
};
