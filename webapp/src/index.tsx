import {Store, Dispatch} from 'redux';
import {GlobalState} from 'mattermost-redux/types/store';

import ListConnectedUsers from 'components/admin_console/get_connected_users_setting';
import InviteWhitelistSetting from 'components/admin_console/invite_whitelist_setting';
import MSTeamsAppManifestSetting from 'components/admin_console/app_manifest_setting';
import {WS_EVENT_USER_CONNECTED, WS_EVENT_USER_DISCONNECTED} from 'types/websocket';
import {userConnectedWsHandler, userDisconnectedWsHandler} from 'websocket_handler';
import {userConnected, userDisconnected} from 'actions';
import {isUserConnected} from 'selectors';
import getUserSettings from 'user_settings';

import Client from './client';
import manifest from './manifest';
import reducer from './reducer';
// eslint-disable-next-line import/no-unresolved
import {PluginRegistry} from './types/mattermost-webapp';
import {getServerRoute} from './selectors';

const MINUTE = 60 * 1000;
const randomInt = (max: number) => Math.floor(Math.random() * max);

export default class Plugin {
    removeStoreSubscription?: () => void;
    activityFunc?: () => void;

    public async initialize(registry: PluginRegistry, store: Store<GlobalState>) {
        const state = store.getState();
        registry.registerReducer(reducer);

        let serverRoute = getServerRoute(state);
        Client.setServerRoute(serverRoute);

        this.fetchUserConnectionStatus(store.dispatch);

        registry.registerAdminConsoleCustomSetting('appManifestDownload', MSTeamsAppManifestSetting);
        registry.registerAdminConsoleCustomSetting('ConnectedUsersReportDownload', ListConnectedUsers);
        registry.registerAdminConsoleCustomSetting('inviteWhitelistUpload', InviteWhitelistSetting);
        this.userActivityWatch();

        registry.registerWebSocketEventHandler(WS_EVENT_USER_CONNECTED, userConnectedWsHandler(store.dispatch));
        registry.registerWebSocketEventHandler(WS_EVENT_USER_DISCONNECTED, userDisconnectedWsHandler(store.dispatch));

        let settingsEnabled = isUserConnected(state);
        registry.registerUserSettings?.(getUserSettings(serverRoute, settingsEnabled));

        this.removeStoreSubscription = store.subscribe(() => {
            const newState = store.getState();
            const newServerRoute = getServerRoute(newState);

            const newSettingsEnabled = isUserConnected(state);
            if (newServerRoute !== serverRoute || newSettingsEnabled !== settingsEnabled) {
                serverRoute = newServerRoute;
                settingsEnabled = newSettingsEnabled;
                registry.registerUserSettings?.(getSettings(serverRoute, settingsEnabled));
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
                    msteams_users_sending: {
                        name: 'MS Teams: Users sending',
                        id: 'msteams_users_sending',
                        icon: 'fa-arrow-up',
                        value: siteStats?.total_users_sending || 0,
                    },
                    msteams_users_receiving: {
                        name: 'MS Teams: Users receiving',
                        id: 'msteams_users_msteams_users_receiving',
                        icon: 'fa-arrow-down',
                        value: siteStats?.total_users_receiving || 0,
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

    uninitialize() {
        this.removeStoreSubscription?.();

        if (this.activityFunc) {
            document.removeEventListener('click', this.activityFunc);
            delete this.activityFunc;
        }
    }
}

declare global {
    interface Window {
        registerPlugin(id: string, plugin: Plugin): void
    }
}

window.registerPlugin(manifest.id, new Plugin());
