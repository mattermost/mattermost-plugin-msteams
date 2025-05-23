// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {Store, Action, Dispatch} from 'redux';

import type {GlobalState} from '@mattermost/types/store';

import {getCurrentUserRoles} from 'mattermost-redux/selectors/entities/users';
import {isGuest} from 'mattermost-redux/utils/user_utils';

import {userConnected, userDisconnected} from './actions';
import Client from './client';
import ConnectedUsersWhitelistSetting from './components/admin_console/connected_users_whitelist_setting';
import ListConnectedUsers from './components/admin_console/get_connected_users_setting';
import manifest from './manifest';
import reducer from './reducer';
import {getServerRoute, isUserConnected} from './selectors';
import {WS_EVENT_USER_CONNECTED, WS_EVENT_USER_DISCONNECTED} from './types/websocket';
import getUserSettings from './user_settings';
import {userConnectedWsHandler, userDisconnectedWsHandler} from './websocket_handler';

import type {PluginRegistry} from '@/types/mattermost-webapp';

const MINUTE = 60 * 1000;
const randomInt = (max: number) => Math.floor(Math.random() * max);

export default class Plugin {
    removeStoreSubscription?: () => void;
    activityFunc?: () => void;

    public async initialize(registry: PluginRegistry, store: Store<GlobalState, Action<Record<string, unknown>>>) {
        const state = store.getState();
        registry.registerReducer(reducer);

        let serverRoute = getServerRoute(state);
        Client.setServerRoute(serverRoute);

        this.fetchUserConnectionStatus(store.dispatch);

        registry.registerAdminConsoleCustomSetting('ConnectedUsersReportDownload', ListConnectedUsers);
        registry.registerAdminConsoleCustomSetting('connectedUsersWhitelist', ConnectedUsersWhitelistSetting);
        this.userActivityWatch();

        registry.registerWebSocketEventHandler(WS_EVENT_USER_CONNECTED, userConnectedWsHandler(store.dispatch));
        registry.registerWebSocketEventHandler(WS_EVENT_USER_DISCONNECTED, userDisconnectedWsHandler(store.dispatch));

        let settingsEnabled = isUserConnected(state);
        registry.registerUserSettings?.(getUserSettings(serverRoute, !settingsEnabled));

        this.removeStoreSubscription = store.subscribe(() => {
            const newState = store.getState();
            const newServerRoute = getServerRoute(newState);
            const newSettingsEnabled = isUserConnected(newState);
            if (newServerRoute !== serverRoute || newSettingsEnabled !== settingsEnabled) {
                serverRoute = newServerRoute;
                settingsEnabled = newSettingsEnabled;
                registry.registerUserSettings?.(getUserSettings(serverRoute, !settingsEnabled));
            }

            if (isGuest(getCurrentUserRoles(newState))) {
                this.stopUserActivityWatch();
            }
        });

        // Site statistics handler
        if (registry.registerSiteStatisticsHandler) {
            registry.registerSiteStatisticsHandler(async () => {
                const siteStats = await Client.fetchSiteStats();
                return {
                    msteams_connected_users: {
                        name: 'MS Teams: Connected Users',
                        id: 'msteams_connected_users',
                        icon: 'fa-users', // font-awesome-4.7.0 handler
                        value: siteStats?.total_connected_users || 0,
                    },
                    msteams_users_active: {
                        name: 'MS Teams: Active Users',
                        id: 'msteams_users_msteams_users_active',
                        icon: 'fa-arrow-down',
                        value: siteStats?.total_active_users || 0,
                    },
                };
            });
        }
    }

    userActivityWatch(): void {
        // Listen for new activity to trigger a call to the server
        // Hat tip to the Github and Playbooks plugin
        let nextCheckAfter = Date.now() + Math.max(MINUTE, randomInt(10 * MINUTE));
        const activityTimeout = 60 * MINUTE; // 1 hour

        this.activityFunc = () => {
            const now = Date.now();
            if (now >= nextCheckAfter) {
                Client.notifyConnect();
                nextCheckAfter = now + activityTimeout;
            }
        };
        document.addEventListener('click', this.activityFunc);
    }

    fetchUserConnectionStatus(dispatch: Dispatch) {
        Client.connectionStatus().then((status) => {
            if (status.connected) {
                dispatch(userConnected());
            } else {
                dispatch(userDisconnected());
            }
        });
    }

    stopUserActivityWatch(): void {
        if (this.activityFunc) {
            document.removeEventListener('click', this.activityFunc);
            delete this.activityFunc;
        }
    }

    uninitialize() {
        this.removeStoreSubscription?.();
        this.stopUserActivityWatch();
    }
}

declare global {
    interface Window {
        registerPlugin(pluginId: string, plugin: Plugin): void;
    }
}

window.registerPlugin(manifest.id, new Plugin());
