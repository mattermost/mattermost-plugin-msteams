import {id as pluginId} from '../manifest';

const getBaseUrls = (): {pluginApiBaseUrl: string} => {
    // TODO: fetch Url from redux
    const url = new URL(window.location.href);
    const baseUrl = `${url.protocol}//${url.host}`;
    const pluginApiBaseUrl = `${baseUrl}/plugins/${pluginId}`;

    return {pluginApiBaseUrl};
};

const getIconUrl = (iconName: string): string => `/plugins/${pluginId}/public/${iconName}`;

// Takes a userId and generates a link to that user's profile image
const getAvatarUrl = (userId: string): string => {
    return `${getBaseUrls().pluginApiBaseUrl}/avatar/${userId}`;
};

export default {getBaseUrls, getIconUrl, getAvatarUrl};
