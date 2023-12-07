import React from 'react';
import {Store, Action} from 'redux';

import {GlobalState} from 'mattermost-redux/types/store';

import reducer from 'reducers';

import EnforceConnectedAccountModal from 'components/enforceConnectedAccountModal';
import MSTeamsAppManifestSetting from 'components/appManifestSetting';
import ListConnectedUsers from 'components/getConnectedUsersSetting';

import {RHSTitle} from 'components';

import {Rhs} from 'containers';

import {pluginTitle} from 'constants/common.constants';

import {iconUrl} from 'constants/illustrations.constants';

import manifest from './manifest';

// eslint-disable-next-line import/no-unresolved
import {PluginRegistry} from './types/mattermost-webapp';
import App from './App';

export default class Plugin {
    enforceConnectedAccountId = '';
    // eslint-disable-next-line @typescript-eslint/no-unused-vars, @typescript-eslint/no-empty-function
    public async initialize(registry: PluginRegistry, store: Store<GlobalState, Action<Record<string, unknown>>>) {
        registry.registerReducer(reducer);
        registry.registerRootComponent(App);

        // @see https://developers.mattermost.com/extend/plugins/webapp/reference/
        this.enforceConnectedAccountId = registry.registerRootComponent(EnforceConnectedAccountModal);

        registry.registerAdminConsoleCustomSetting('appManifestDownload', MSTeamsAppManifestSetting);
        registry.registerAdminConsoleCustomSetting('ConnectedUsersReportDownload', ListConnectedUsers);
        const {_, toggleRHSPlugin} = registry.registerRightHandSidebarComponent(Rhs, <RHSTitle/>);

        // TODO: update icons later
        registry.registerChannelHeaderButtonAction(
            <img
                width={24}
                height={24}
                src={iconUrl}
            />, () => store.dispatch(toggleRHSPlugin), null, pluginTitle);

        if (registry.registerAppBarComponent) {
            registry.registerAppBarComponent(iconUrl, () => store.dispatch(toggleRHSPlugin), pluginTitle);
        }
    }
}

declare global {
    interface Window {
        registerPlugin(id: string, plugin: Plugin): void,
    }
}

window.registerPlugin(manifest.id, new Plugin());
