import {createApi, fetchBaseQuery} from '@reduxjs/toolkit/query/react';

import Cookies from 'js-cookie';

import Constants from '../constants';
import utils from '../utils';

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
        [Constants.pluginApiServiceConfigs.needsConnect.apiServiceName]: builder.query<NeedsConnectData, APIRequestPayload>({
            query: () => ({
                url: Constants.pluginApiServiceConfigs.needsConnect.path,
                method: Constants.pluginApiServiceConfigs.needsConnect.method,
            }),
        }),
        [Constants.pluginApiServiceConfigs.connect.apiServiceName]: builder.query<ConnectData, APIRequestPayload>({
            query: () => ({
                url: Constants.pluginApiServiceConfigs.connect.path,
                method: Constants.pluginApiServiceConfigs.connect.method,
            }),
        }),
        [Constants.pluginApiServiceConfigs.getLinkedChannels.apiServiceName]: builder.query<ChannelLinkData[], APIRequestPayload>({
            query: (params) => ({
                url: Constants.pluginApiServiceConfigs.getLinkedChannels.path,
                method: Constants.pluginApiServiceConfigs.getLinkedChannels.method,
                params: {...params},
            }),
        }),
    }),
});
