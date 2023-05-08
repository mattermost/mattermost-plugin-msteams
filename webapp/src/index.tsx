import {Store, Action} from 'redux';

import {GlobalState} from 'mattermost-redux/types/store';

import manifest from './manifest';
import Client from './client';
import EnforceConnectedAccountModal from './components/enforceConnectedAccountModal';

// eslint-disable-next-line import/no-unresolved
import {PluginRegistry} from './types/mattermost-webapp';
import {getServerRoute} from './selectors';

export default class Plugin {
    enforceConnectedAccountId = '';
    // eslint-disable-next-line @typescript-eslint/no-unused-vars, @typescript-eslint/no-empty-function
    public async initialize(registry: PluginRegistry, store: Store<GlobalState, Action<Record<string, unknown>>>) {
        Client.setServerRoute(getServerRoute(store.getState()));

        // @see https://developers.mattermost.com/extend/plugins/webapp/reference/
        this.enforceConnectedAccountId = registry.registerRootComponent(EnforceConnectedAccountModal);
    }
}

declare global {
    interface Window {
        registerPlugin(id: string, plugin: Plugin): void
    }
}

window.registerPlugin(manifest.id, new Plugin());
