import {Store, Action} from 'redux';

import {GlobalState} from 'mattermost-redux/types/store';

import manifest from './manifest';
import Client from './client';
import ListConnectedUsers from './components/getConnectedUsersSetting';
import EnforceConnectedAccountModal from './components/enforceConnectedAccountModal';
import MSTeamsAppManifestSetting from './components/appManifestSetting';

// eslint-disable-next-line import/no-unresolved
import {PluginRegistry} from './types/mattermost-webapp';
import {getServerRoute} from './selectors';

export default class Plugin {
    enforceConnectedAccountId = '';
    // eslint-disable-next-line @typescript-eslint/no-unused-vars, @typescript-eslint/no-empty-function
    public async initialize(registry: PluginRegistry, store: Store<GlobalState, Action<Record<string, unknown>>>) {
        const serverRoute = getServerRoute(store.getState());
        Client.setServerRoute(serverRoute);

        // @see https://developers.mattermost.com/extend/plugins/webapp/reference/
        this.enforceConnectedAccountId = registry.registerRootComponent(EnforceConnectedAccountModal);

        registry.registerAdminConsoleCustomSetting('appManifestDownload', MSTeamsAppManifestSetting);
        registry.registerAdminConsoleCustomSetting('ConnectedUsersReportDownload', ListConnectedUsers);
        registry.registerUserSettings?.({
            id: manifest.id,
            icon: `${serverRoute}/plugins/${manifest.id}/public/msteams-sync-icon.svg`,
            uiName: manifest.name,
            sections: [{
                settings: [{
                    name: 'primary_platform',
                    options: [
                        {
                            text: 'Mattermost will be my primary platform',
                            value: 'mm',
                            helpText: 'You will get notifications in Mattermost for synced messages and channels. You will need to disable notifications in Microsoft Teams to avoid duplicates. **[Learn more](http://google.com)**',
                        },
                        {
                            text: 'Microsoft Teams will be my primary platform',
                            value: 'teams',
                            helpText: 'Notifications in Mattermost will be muted for linked channels and DMs to prevent duplicates. You can unmute any linked channel or DM/GM if you with to receive notifications. **[Learn more](http://google.com)**',
                        },
                    ],
                    type: 'radio',
                    default: 'mm',
                    helpText: 'Note: Unread statuses for linked channels and DMs will not be synced between Mattermost & Microsoft Teams.',
                }],
                title: 'Primary platform for communication',
            }],
        });
    }
}

declare global {
    interface Window {
        registerPlugin(id: string, plugin: Plugin): void
    }
}

window.registerPlugin(manifest.id, new Plugin());
