import {GlobalState} from 'mattermost-redux/types/store';

import {getConfig} from 'mattermost-redux/selectors/entities/general';

import {pluginId} from 'manifest';
import {PluginState} from 'reducer';

export const getServerRoute = (state: GlobalState) => {
    const config = getConfig(state);

    let basePath = '';
    if (config && config.SiteURL) {
        basePath = new URL(config.SiteURL).pathname;

        if (basePath && basePath[basePath.length - 1] === '/') {
            basePath = basePath.substr(0, basePath.length - 1);
        }
    }

    return basePath;
};

const pluginState = (state: GlobalState): PluginState => state['plugins-' + pluginId as keyof GlobalState] as unknown as PluginState || {} as PluginState;

export const isUserConnected = (state: GlobalState): boolean => pluginState(state).userConnected;
