import React from 'react';
import {Store, Action} from 'redux';

import {GlobalState} from 'mattermost-redux/types/store';

import reducer from 'reducers';

import EnforceConnectedAccountModal from 'components/enforceConnectedAccountModal';
import MSTeamsAppManifestSetting from 'components/appManifestSetting';
import ListConnectedUsers from 'components/getConnectedUsersSetting';

import {handleConnect, handleDisconnect} from 'websocket';

import manifest from './manifest';

import App from './App';

export default class Plugin {
    enforceConnectedAccountId = '';
    public async initialize(registry: PluginRegistry, store: Store<GlobalState, Action<Record<string, unknown>>>) {
        registry.registerReducer(reducer);
        registry.registerRootComponent(() => (
            <App
                registry={registry}
                store={store}
            />
        ));

        // @see https://developers.mattermost.com/extend/plugins/webapp/reference/
        this.enforceConnectedAccountId = registry.registerRootComponent(EnforceConnectedAccountModal);

        registry.registerAdminConsoleCustomSetting('appManifestDownload', MSTeamsAppManifestSetting);
        registry.registerAdminConsoleCustomSetting('ConnectedUsersReportDownload', ListConnectedUsers);

        registry.registerWebSocketEventHandler(`custom_${manifest.id}_connect`, handleConnect(store));
        registry.registerWebSocketEventHandler(`custom_${manifest.id}_disconnect`, handleDisconnect(store));
    }
}

declare global {
    interface Window {
        registerPlugin(id: string, plugin: Plugin): void,
    }
}

window.registerPlugin(manifest.id, new Plugin());
