import {id as pluginId} from '../manifest';

const iconUrl = `/plugins/${pluginId}/public/msteams-sync-icon.svg`;
const pluginTitle = 'Microsoft Teams Sync';
const siteUrl = 'SITEURL';
import {pluginApiServiceConfigs} from './apiService';

export default {
    iconUrl,
    pluginTitle,
    siteUrl,
    pluginApiServiceConfigs,
};
