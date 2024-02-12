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
        icon: `${serverRoute}/plugins/${manifest.id}/public/msteams-sync-icon.svg`,
        uiName: manifest.name,
        action: disabled ?
            {
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
                        helpText: 'You will get notifications in Mattermost for synced messages and channels. You will need to disable notifications in Microsoft Teams to avoid duplicates. **[Learn more](http://google.com)**',
                    },
                    {
                        text: 'Microsoft Teams',
                        value: 'msteams',
                        helpText: 'Notifications in Mattermost will be muted for linked channels and DMs to prevent duplicates. You can unmute any linked channel or DM/GM if you wish to receive notifications. **[Learn more](http://google.com)**',
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
        const serverRoute = getServerRoute(state);
        Client.setServerRoute(serverRoute);

        registry.registerAdminConsoleCustomSetting('appManifestDownload', MSTeamsAppManifestSetting);
        registry.registerAdminConsoleCustomSetting('ConnectedUsersReportDownload', ListConnectedUsers);

        // TODO to uncomment when the feature is in.
        // let settingsEnabled = (state as any)[`plugins-${manifest.id}`]?.connectedStateSlice?.connected || false; //TODO use connected selector from https://github.com/mattermost/mattermost-plugin-msteams-sync/pull/438
        // registry.registerUserSettings?.(getSettings(serverRoute, !settingsEnabled));

        // this.removeStoreSubscription = store.subscribe(() => {
        //     const newState = store.getState();
        //     const newServerRoute = getServerRoute(newState);
        //     const newSettingsEnabled = (newState as any)[`plugins-${manifest.id}`]?.connectedStateSlice?.connected || false; //TODO use connected selector from https://github.com/mattermost/mattermost-plugin-msteams-sync/pull/438
        //     if (newServerRoute !== serverRoute || newSettingsEnabled !== settingsEnabled) {
        //         serverRoute = newServerRoute;
        //         settingsEnabled = newSettingsEnabled;
        //         registry.registerUserSettings?.(getSettings(serverRoute, !settingsEnabled));
        //     }
        // });
    }

    // uninitialize() {
    //     this.removeStoreSubscription?.();
    // }
}

declare global {
    interface Window {
        registerPlugin(id: string, plugin: Plugin): void
    }
}

window.registerPlugin(manifest.id, new Plugin());
