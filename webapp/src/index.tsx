import {Store, Action} from 'redux';

import {GlobalState} from 'mattermost-redux/types/store';

import manifest from './manifest';
import Client from './client';
import ListConnectedUsers from './components/getConnectedUsersSetting';
import MSTeamsAppManifestSetting from './components/appManifestSetting';

// eslint-disable-next-line import/no-unresolved
import {PluginRegistry} from './types/mattermost-webapp';
import {getServerRoute} from './selectors';

function getSettings(serverRoute: string, disabled: boolean) {
    return {
        id: manifest.id,
        icon: `${serverRoute}/plugins/${manifest.id}/public/icon.svg`,
        uiName: manifest.name,
        action: disabled ?
            {
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

    public async initialize(registry: PluginRegistry, store: Store<GlobalState, Action<Record<string, unknown>>>) {
        const state = store.getState();
        let serverRoute = getServerRoute(state);
        Client.setServerRoute(serverRoute);

        registry.registerAdminConsoleCustomSetting('appManifestDownload', MSTeamsAppManifestSetting);
        registry.registerAdminConsoleCustomSetting('ConnectedUsersReportDownload', ListConnectedUsers);

        // let settingsEnabled = (state as any)[`plugins-${manifest.id}`]?.connectedStateSlice?.connected || false; //TODO use connected selector from https://github.com/mattermost/mattermost-plugin-msteams/pull/438
        let settingsEnabled = false;
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
    }

    uninitialize() {
        this.removeStoreSubscription?.();
    }
}

declare global {
    interface Window {
        registerPlugin(id: string, plugin: Plugin): void
    }
}

window.registerPlugin(manifest.id, new Plugin());
