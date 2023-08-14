type HttpMethod = 'GET' | 'POST' | 'PATCH' | 'DELETE';

type PluginApiServiceName =
    'needsConnect' |
    'connect' |
    'getLinkedChannels' |
    'disconnectUser';

type PluginApiService = {
    path: string,
    method: httpMethod,
    apiServiceName: PluginApiServiceName,
}

type APIError = {
    status: string,
    data: string,
}

type APIRequestPayload = PaginationQueryParams | void;
