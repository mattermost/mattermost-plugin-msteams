import {id as pluginId} from '../manifest';

const iconUrl = `/plugins/${pluginId}/public/msteams-sync-icon.svg`;
const notConnectIconUrl = `/plugins/${pluginId}/public/msteams-sync-connect-icon.svg`;
const linkIconUrl = `/plugins/${pluginId}/public/msteams-sync-link-icon.svg`;
const globeIconUrl = `/plugins/${pluginId}/public/msteams-sync-globe-icon.svg`;
const msteamsIconUrl = `/plugins/${pluginId}/public/msteams-icon.svg`;
const channelUnlinkIconUrl = `/plugins/${pluginId}/public/msteams-channel-unlink-icon.svg`;
const mmPublicChannelIconUrl = `/plugins/${pluginId}/public/mm-public-channel-icon.svg`;
const mmPrivateChannelIconUrl = `/plugins/${pluginId}/public/mm-private-channel-icon.svg`;
const pluginTitle = 'Microsoft Teams Sync';
const siteUrl = 'SITEURL';

const DefaultPage = 0;
const DefaultPageSize = 20;

import {pluginApiServiceConfigs} from './apiService';

export default {
    iconUrl,
    notConnectIconUrl,
    globeIconUrl,
    linkIconUrl,
    msteamsIconUrl,
    channelUnlinkIconUrl,
    mmPrivateChannelIconUrl,
    mmPublicChannelIconUrl,
    pluginTitle,
    siteUrl,
    DefaultPage,
    DefaultPageSize,
    pluginApiServiceConfigs,
};
