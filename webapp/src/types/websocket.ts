// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {pluginId} from 'manifest';

export const WS_EVENT_USER_CONNECTED = 'custom_' + pluginId + '_user_connected';
export const WS_EVENT_USER_DISCONNECTED = 'custom_' + pluginId + '_user_disconnected';
