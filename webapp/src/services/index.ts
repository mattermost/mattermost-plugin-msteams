import {createApi, fetchBaseQuery} from '@reduxjs/toolkit/query/react';

// eslint-disable-next-line import/no-unresolved
import Cookies from 'js-cookie';

import {pluginApiServiceConfigs} from 'constants/apiService.constant';

import utils from 'utils';

// Service to make plugin API requests
export const msTeamsPluginApi = createApi({
    reducerPath: 'msTeamsPluginApi',
    baseQuery: fetchBaseQuery({
        baseUrl: utils.getBaseUrls().pluginApiBaseUrl,
        prepareHeaders: (headers) => {
            headers.set('X-CSRF-Token', Cookies.get('MMCSRF') ?? '');
            return headers;
        },
    }),
    endpoints: (builder) => ({
        [pluginApiServiceConfigs.needsConnect.apiServiceName]: builder.query<NeedsConnectData, APIRequestPayload>({
            query: () => ({
                url: pluginApiServiceConfigs.needsConnect.path,
                method: pluginApiServiceConfigs.needsConnect.method,
            }),
        }),
        [pluginApiServiceConfigs.connect.apiServiceName]: builder.query<ConnectData, APIRequestPayload>({
            query: () => ({
                url: pluginApiServiceConfigs.connect.path,
                method: pluginApiServiceConfigs.connect.method,
            }),
        }),
        [pluginApiServiceConfigs.whitelistUser.apiServiceName]: builder.query<WhitelistUserResponse, APIRequestPayload>({
            query: () => ({
                url: pluginApiServiceConfigs.whitelistUser.path,
                method: pluginApiServiceConfigs.whitelistUser.method,
            }),
        }),
        [pluginApiServiceConfigs.getLinkedChannels.apiServiceName]: builder.query<ChannelLinkData[], APIRequestPayload>({
            query: (params) => ({
                url: pluginApiServiceConfigs.getLinkedChannels.path,
                method: pluginApiServiceConfigs.getLinkedChannels.method,
                params: {...params},
            }),
        }),
        [pluginApiServiceConfigs.disconnectUser.apiServiceName]: builder.query<string, APIRequestPayload>({
            query: () => ({
                url: pluginApiServiceConfigs.disconnectUser.path,
                method: pluginApiServiceConfigs.disconnectUser.method,
                responseHandler: (res) => res.text(),
            }),
        }),
    }),
});
