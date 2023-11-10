type HttpMethod = 'GET' | 'POST' | 'PATCH' | 'DELETE';

type PluginApiServiceName =
    'needsConnect' |
    'connect' |
    'getLinkedChannels' |
    'disconnectUser' |
    'unlinkChannel' |
    'searchMSTeams' |
    'searchMSChannels'|
    'linkChannels';

type PluginApiService = {
    path: string,
    method: httpMethod,
    apiServiceName: PluginApiServiceName,
}

type APIError = {
    status: string,
    data: string,
}

type APIRequestPayload = PaginationQueryParams | UnlinkChannelParams | SearchParams | LinkChannelsPayload | void;
