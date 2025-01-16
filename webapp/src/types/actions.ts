// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {pluginId} from '../manifest';

export const USER_CONNECTED = pluginId + '_user_connected';
export const USER_DISCONNECTED = pluginId + '_user_disconnected';

export interface UserHasConnected {
    type: typeof USER_CONNECTED;
}

export interface UserHasDisconnected {
    type: typeof USER_DISCONNECTED;
}
