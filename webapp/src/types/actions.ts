import {pluginId} from '../manifest';

export const USER_HAS_CONNECTED = pluginId + '_user_has_connected';
export const USER_HAS_DISCONNECTED = pluginId + '_user_has_disconnected';

export interface UserHasConnected {
    type: typeof USER_HAS_CONNECTED;
}

export interface UserHasDisconnected {
    type: typeof USER_HAS_DISCONNECTED;
}
