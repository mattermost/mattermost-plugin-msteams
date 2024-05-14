import {Store, Action} from 'redux';

import {GlobalState} from 'mattermost-redux/types/store';

import ListConnectedUsers from 'components/admin_console/get_connected_users_setting';
import InviteWhitelistSetting from 'components/admin_console/invite_whitelist_setting';
import MSTeamsAppManifestSetting from 'components/admin_console/app_manifest_setting';

import Client from './client';
import manifest from './manifest';

// eslint-disable-next-line import/no-unresolved
import {PluginRegistry} from './types/mattermost-webapp';
import {getServerRoute} from './selectors';

const MINUTE = 60 * 1000;
const randomInt = (max: number) => Math.floor(Math.random() * max);

function getSettings(serverRoute: string, disabled: boolean) {
    return {
        id: manifest.id,
        icon: `${serverRoute}/plugins/${manifest.id}/public/icon.svg`,
        uiName: manifest.name,
        action: disabled ? {
            title: 'Connect your Microsoft Teams Account',
            text: 'Connect your Mattermost and Microsoft Teams accounts to get the ability to link and synchronise channel-based collaboration with Microsoft Teams.',
            buttonText: 'Connect account',
            onClick: () => window.open(`${Client.url}/connect`),
        } : undefined, //eslint-disable-line no-undefined
        sections: [{
            settings: [{
                name: 'platform',
                options: [
                    {
                        text: 'Mattermost',
                        value: 'mattermost',
                        helpText: 'You will get notifications in Mattermost for synced messages and channels. You will need to disable notifications in Microsoft Teams to avoid duplicates. **[Learn more](https://mattermost.com/pl/ms-teams-plugin-end-user-learn-more)**',
                    },
                    {
                        text: 'Microsoft Teams',
                        value: 'msteams',
                        helpText: 'Notifications in Mattermost will be muted for linked channels and DMs to prevent duplicates. You can unmute any linked channel or DM/GM if you wish to receive notifications. **[Learn more](https://mattermost.com/pl/ms-teams-plugin-end-user-learn-more)**',
                    },
                ],
                type: 'radio' as const,
                default: 'mattermost',
                helpText: 'Note: Unread statuses for linked channels and DMs will not be synced between Mattermost & Microsoft Teams.',
            }],
            title: 'Primary platform for communication',
            disabled,
        }],
    };
}

export default class Plugin {
    removeStoreSubscription?: () => void;
    activityFunc?: () => void;

    public async initialize(registry: PluginRegistry, store: Store<GlobalState, Action<Record<string, unknown>>>) {
        const state = store.getState();
        let serverRoute = getServerRoute(state);
        Client.setServerRoute(serverRoute);

        registry.registerAdminConsoleCustomSetting('appManifestDownload', MSTeamsAppManifestSetting);
        registry.registerAdminConsoleCustomSetting('ConnectedUsersReportDownload', ListConnectedUsers);
        registry.registerAdminConsoleCustomSetting('inviteWhitelistUpload', InviteWhitelistSetting);
        this.userActivityWatch();

        // let settingsEnabled = (state as any)[`plugins-${manifest.id}`]?.connectedStateSlice?.connected || false; //TODO use connected selector from https://github.com/mattermost/mattermost-plugin-msteams/pull/438
        let settingsEnabled = true;
        registry.registerUserSettings?.(getSettings(serverRoute, !settingsEnabled));

        this.removeStoreSubscription = store.subscribe(() => {
            const newState = store.getState();
            const newServerRoute = getServerRoute(newState);

            // const newSettingsEnabled = (newState as any)[`plugins-${manifest.id}`]?.connectedStateSlice?.connected || false; //TODO use connected selector from https://github.com/mattermost/mattermost-plugin-msteams/pull/438
            const newSettingsEnabled = true;
            if (newServerRoute !== serverRoute || newSettingsEnabled !== settingsEnabled) {
                serverRoute = newServerRoute;
                settingsEnabled = newSettingsEnabled;
                registry.registerUserSettings?.(getSettings(serverRoute, !settingsEnabled));
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
                        value: siteStats?.total_connected_users,
                    },
                    msteams_invited_users: {
                        name: 'MS Teams: Invited Users',
                        id: 'msteams_invited_users',
                        icon: 'fa-users', // font-awesome-4.7.0 handler
                        value: siteStats?.pending_invited_users,
                    },
                    msteams_whitelisted_users: {
                        name: 'MS Teams: Whitelisted Users',
                        id: 'msteams_whitelisted_users',
                        icon: 'fa-users', // font-awesome-4.7.0 handler
                        value: siteStats?.total_connected_users,
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
