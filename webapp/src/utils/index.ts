import {id as pluginId} from '../manifest';

const getBaseUrls = (): {pluginApiBaseUrl: string} => {
    const url = new URL(window.location.href);
    const baseUrl = `${url.protocol}//${url.host}`;
    const pluginApiBaseUrl = `${baseUrl}/plugins/${pluginId}`;

    return {pluginApiBaseUrl};
};

export default {getBaseUrls};
