import React from 'react';
import {Store, Action} from 'redux';

import {GlobalState} from 'mattermost-redux/types/store';

import LinkChannels from 'src/containers/linkChannels';

import {handleConnect, handleDisconnect, handleOpenLinkChannelsModal} from 'src/websocket';

import Rhs from 'src/containers/Rhs';

import Constants from 'src/constants';

import reducer from 'src/reducers';

import manifest from 'src/manifest';

import EnforceConnectedAccountModal from 'src/components/enforceConnectedAccountModal';
import MSTeamsAppManifestSetting from 'src/components/appManifestSetting';
import App from 'src/app';

// eslint-disable-next-line import/no-unresolved
import {PluginRegistry} from './types/mattermost-webapp';

export default class Plugin {
    enforceConnectedAccountId = '';
    // eslint-disable-next-line @typescript-eslint/no-unused-vars, @typescript-eslint/no-empty-function
    public async initialize(registry: PluginRegistry, store: Store<GlobalState, Action<Record<string, unknown>>>) {
        registry.registerReducer(reducer);
        registry.registerRootComponent(LinkChannels);

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
        registry.registerWebSocketEventHandler(`custom_${manifest.id}_link_channels`, handleOpenLinkChannelsModal(store));
    }
}

declare global {
    interface Window {
        registerPlugin(id: string, plugin: Plugin): void
    }
}

window.registerPlugin(manifest.id, new Plugin());
