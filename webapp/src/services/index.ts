import {createApi, fetchBaseQuery} from '@reduxjs/toolkit/query/react';

// eslint-disable-next-line import/no-unresolved
import {BaseQueryApi} from '@reduxjs/toolkit/dist/query/baseQueryTypes';

// eslint-disable-next-line import/no-unresolved
import Cookies from 'js-cookie';

import {GlobalState} from 'mattermost-redux/types/store.d';

import {pluginApiServiceConfigs} from 'constants/apiService.constant';

import utils from 'utils';

const handleBaseQuery = async (
    args: {
        url: string,
        method: string,
    },
    api: BaseQueryApi,
    extraOptions: Record<string, string> = {},
) => {
    const globalReduxState = api.getState() as GlobalState;
    const result = await fetchBaseQuery({
        baseUrl: utils.getBaseUrls(globalReduxState?.entities?.general?.config?.SiteURL).pluginApiBaseUrl,
        prepareHeaders: (headers) => {
            headers.set('X-CSRF-Token', Cookies.get('MMCSRF') ?? '');
            return headers;
        },
    })(
        args,
        api,
        extraOptions,
    );
    return result;
};

// Service to make plugin API requests
export const msTeamsPluginApi = createApi({
    reducerPath: 'msTeamsPluginApi',
    baseQuery: handleBaseQuery,
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
        [pluginApiServiceConfigs.getConfig.apiServiceName]: builder.query<ConfigResponse, APIRequestPayload>({
            query: () => ({
                url: pluginApiServiceConfigs.getConfig.path,
                method: pluginApiServiceConfigs.getConfig.method,
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
                responseHandler: (res: Response) => res.text(),
            }),
        }),
        [pluginApiServiceConfigs.searchMSTeams.apiServiceName]: builder.query<MSTeamsSearchResponse, SearchParams>({
            query: (params) => ({
                url: pluginApiServiceConfigs.searchMSTeams.path,
                method: pluginApiServiceConfigs.searchMSTeams.method,
                params: {...params},
            }),
        }),
        [pluginApiServiceConfigs.searchMSChannels.apiServiceName]: builder.query<MSTeamsSearchResponse, SearchMSChannelsParams>({
            query: ({teamId, ...params}) => ({
                url: pluginApiServiceConfigs.searchMSChannels.path.replace('{team_id}', teamId),
                method: pluginApiServiceConfigs.searchMSChannels.method,
                params: {...params},
            }),
        }),
        [pluginApiServiceConfigs.linkChannels.apiServiceName]: builder.query<string, LinkChannelsPayload>({
            query: (payload) => ({
                url: pluginApiServiceConfigs.linkChannels.path,
                method: pluginApiServiceConfigs.linkChannels.method,
                body: payload,
                responseHandler: (res: Response) => res.text(),
            }),
        }),
        [pluginApiServiceConfigs.unlinkChannel.apiServiceName]: builder.query<string, UnlinkChannelParams>({
            query: ({channelId}) => ({
                url: pluginApiServiceConfigs.unlinkChannel.path.replace('{channel_id}', channelId),
                method: pluginApiServiceConfigs.unlinkChannel.method,
                responseHandler: (res: Response) => res.text(),
            }),
        }),
    }),
});
