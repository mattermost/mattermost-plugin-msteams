import {Store, Action} from 'redux';

import {GlobalState} from 'mattermost-redux/types/store';

import {useEffect} from 'react';

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
        action: disabled ? {
            title: 'Connect your Microsoft Teams Account',
            text: 'Connect your Mattermost and Microsoft Teams accounts to get the ability to link and synchronise channel-based collaboration with Microsoft Teams.',
            buttonText: 'Connect account',
            onClick: () => Client.connect().then((result) => {
                window.open(result?.connectUrl, '_blank');
            }),
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
        registry.registerRootComponent(RootEffects);

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
    }

    userActivityWatch(): void {
        // Listen for new activity to trigger a call to the server
        // Hat tip to the Github and Playbooks plugin
        let lastActivityTime = Number.MAX_SAFE_INTEGER;
        const activityTimeout = 60 * 60 * 1000; // 1 hour

        this.activityFunc = () => {
            const now = new Date().getTime();
            if (now - lastActivityTime > activityTimeout) {
                Client.notifyConnect();
            }
            lastActivityTime = now;
        };
        document.addEventListener('click', this.activityFunc);
    }

    uninitialize() {
        this.removeStoreSubscription?.();
    }
}

const RootEffects = () => {
    useEffect(() => {
        Client.notifyConnect();
    }, []);

    return null;
};

declare global {
    interface Window {
        registerPlugin(id: string, plugin: Plugin): void
    }
}

window.registerPlugin(manifest.id, new Plugin());
