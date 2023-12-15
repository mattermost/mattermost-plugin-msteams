import {id as pluginId} from '../manifest';

const getBaseUrls = (siteURL?: string): {pluginApiBaseUrl: string} => {
    const pluginApiBaseUrl = `${siteURL}/plugins/${pluginId}`;
    return {pluginApiBaseUrl};
};

const getIconUrl = (iconName: string): string => `/plugins/${pluginId}/public/${iconName}`;

// Takes a userId and generates a link to that user's profile image
const getAvatarUrl = (userId: string, siteURL: string): string => {
    return `${getBaseUrls(siteURL).pluginApiBaseUrl}/avatar/${userId}`;
};

export default {getBaseUrls, getIconUrl, getAvatarUrl};
