import React from 'react';
import {Store, Action} from 'redux';

import {GlobalState} from 'mattermost-redux/types/store';

import {handleConnect, handleDisconnect} from './websocket';

import Rhs from './containers/Rhs';
import Constants from './constants';
import reducer from './reducers';

import manifest from './manifest';

import EnforceConnectedAccountModal from './components/enforceConnectedAccountModal';
import MSTeamsAppManifestSetting from './components/appManifestSetting';
import App from './app';

// eslint-disable-next-line import/no-unresolved
import {PluginRegistry} from './types/mattermost-webapp';

export default class Plugin {
    enforceConnectedAccountId = '';
    // eslint-disable-next-line @typescript-eslint/no-unused-vars, @typescript-eslint/no-empty-function
    public async initialize(registry: PluginRegistry, store: Store<GlobalState, Action<Record<string, unknown>>>) {
        registry.registerReducer(reducer);

        // @see https://developers.mattermost.com/extend/plugins/webapp/reference/
        this.enforceConnectedAccountId = registry.registerRootComponent(EnforceConnectedAccountModal);
        registry.registerRootComponent(App);

        registry.registerAdminConsoleCustomSetting('appManifestDownload', MSTeamsAppManifestSetting);
        const {_, toggleRHSPlugin} = registry.registerRightHandSidebarComponent(Rhs, Constants.pluginTitle);

        // TODO: update icons later
        registry.registerChannelHeaderButtonAction(
            <img
                width={24}
                height={24}
                src={Constants.iconUrl}
            />, () => store.dispatch(toggleRHSPlugin), null, Constants.pluginTitle);
        if (registry.registerAppBarComponent) {
            registry.registerAppBarComponent(Constants.iconUrl, () => store.dispatch(toggleRHSPlugin), Constants.pluginTitle);
        }

        registry.registerWebSocketEventHandler(`custom_${manifest.id}_connect`, handleConnect(store));
        registry.registerWebSocketEventHandler(`custom_${manifest.id}_disconnect`, handleDisconnect(store));
    }
}

declare global {
    interface Window {
        registerPlugin(id: string, plugin: Plugin): void
    }
}

window.registerPlugin(manifest.id, new Plugin());
