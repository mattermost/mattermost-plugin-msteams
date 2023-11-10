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
        [Constants.pluginApiServiceConfigs.disconnectUser.apiServiceName]: builder.query<string, APIRequestPayload>({
            query: () => ({
                url: Constants.pluginApiServiceConfigs.disconnectUser.path,
                method: Constants.pluginApiServiceConfigs.disconnectUser.method,
                responseHandler: (res) => res.text(),
            }),
        }),
        [Constants.pluginApiServiceConfigs.unlinkChannel.apiServiceName]: builder.query<string, UnlinkChannelParams>({
            query: ({channelId}) => ({
                url: `${Constants.pluginApiServiceConfigs.unlinkChannel.path}/${channelId}`,
                method: Constants.pluginApiServiceConfigs.unlinkChannel.method,
                responseHandler: (res) => res.text(),
            }),
        }),
        [Constants.pluginApiServiceConfigs.searchMSTeams.apiServiceName]: builder.query<MSTeamsSearchResponse, SearchParams>({
            query: (params) => ({
                url: Constants.pluginApiServiceConfigs.searchMSTeams.path,
                method: Constants.pluginApiServiceConfigs.searchMSTeams.method,
                params: {...params},
            }),
        }),
        [Constants.pluginApiServiceConfigs.searchMSChannels.apiServiceName]: builder.query<MSTeamsSearchResponse, SearchMSChannelsParams>({
            query: ({teamId, ...params}) => ({
                url: Constants.pluginApiServiceConfigs.searchMSChannels.path.replace('{team_id}', teamId),
                method: Constants.pluginApiServiceConfigs.searchMSChannels.method,
                params: {...params},
            }),
        }),
        [Constants.pluginApiServiceConfigs.linkChannels.apiServiceName]: builder.query<string, LinkChannelsPayload>({
            query: (payload) => ({
                url: Constants.pluginApiServiceConfigs.linkChannels.path,
                method: Constants.pluginApiServiceConfigs.linkChannels.method,
                body: payload,
                responseHandler: (res) => res.text(),
            }),
        }),
    }),
});
