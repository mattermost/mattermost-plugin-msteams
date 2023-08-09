import {id as pluginId} from '../manifest';

const iconUrl = `/plugins/${pluginId}/public/msteams-sync-icon.svg`;
const pluginTitle = 'Microsoft Teams Sync';
const siteUrl = 'SITEURL';

const DefaultPage = 0;
const DefaultPageSize = 20;

import {pluginApiServiceConfigs} from './apiService';

export default {
    iconUrl,
    pluginTitle,
    siteUrl,
    DefaultPage,
    DefaultPageSize,
    pluginApiServiceConfigs,
};
