type HttpMethod = 'GET' | 'POST' | 'PATCH' | 'DELETE';

type PluginApiServiceName = 'needsConnect' | 'connect' | 'getConfig' | 'getLinkedChannels' | 'disconnectUser' | 'searchMSTeams' | 'searchMSChannels' | 'linkChannels' | 'unlinkChannel';

type PluginApiService = {
    path: string,
    method: httpMethod,
    apiServiceName: PluginApiServiceName,
}

type APIError = {
    id: string,
    message: string,
}

type APIRequestPayload = LinkChannelsPayload | SearchParams | UnlinkChannelParams | void;
