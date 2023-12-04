type HttpMethod = 'GET' | 'POST' | 'PATCH' | 'DELETE';

type PluginApiServiceName = 'needsConnect' | 'connect' | 'whitelistUser' | 'getLinkedChannels';

type PluginApiService = {
    path: string,
    method: httpMethod,
    apiServiceName: PluginApiServiceName,
}

type APIError = {
    id: string,
    message: string,
}

type APIRequestPayload = SearchLinkedChannelParams | void;
