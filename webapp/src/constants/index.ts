import {id as pluginId} from '../manifest';

const iconUrl = `/plugins/${pluginId}/public/msteams-sync-icon.svg`;
const mattermostIconUrl = `/plugins/${pluginId}/public/mattermost-icon.svg`;
const mattermostHollowIconUrl = `/plugins/${pluginId}/public/mattermost-icon-hollow.svg`;
const msteamsPrivateChannelIconUrl = `/plugins/${pluginId}/public/msteams-private-channel-icon.svg`;
const msteamsIconUrl = `/plugins/${pluginId}/public/msteams-icon.svg`;
const checkIconUrl = `/plugins/${pluginId}/public/check.svg`;
const errorIconUrl = `/plugins/${pluginId}/public/error.svg`;
const pluginTitle = 'Microsoft Teams Sync';
const siteUrl = 'SITEURL';

const DefaultPage = 0;
const DefaultPerPage = 20;
const DebounceFunctionTimeLimit = 500;

import {pluginApiServiceConfigs} from './apiService';

export enum ModalIds {
    LINK_CHANNELS = 'linkChannels',
}

export default {
    iconUrl,
    mattermostIconUrl,
    mattermostHollowIconUrl,
    msteamsPrivateChannelIconUrl,
    msteamsIconUrl,
    checkIconUrl,
    errorIconUrl,
    pluginTitle,
    siteUrl,
    DefaultPage,
    DefaultPerPage,
    DebounceFunctionTimeLimit,
    pluginApiServiceConfigs,
};
