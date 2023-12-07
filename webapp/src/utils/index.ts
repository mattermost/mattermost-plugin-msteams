import {id as pluginId} from '../manifest';

const getBaseUrls = (): {pluginApiBaseUrl: string} => {
    // TODO: fetch Url from redux
    const url = new URL(window.location.href);
    const baseUrl = `${url.protocol}//${url.host}`;
    const pluginApiBaseUrl = `${baseUrl}/plugins/${pluginId}`;

    return {pluginApiBaseUrl};
};

const getIconUrl = (iconName: string): string => `/plugins/${pluginId}/public/${iconName}`;

export default {getBaseUrls, getIconUrl};
