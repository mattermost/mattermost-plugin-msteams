// Plugin api service (RTK query) configs
export const pluginApiServiceConfigs: Record<PluginApiServiceName, PluginApiService> = {
    connect: {
        path: '/connect',
        method: 'GET',
        apiServiceName: 'connect',
    },
    needsConnect: {
        path: '/needsConnect',
        method: 'GET',
        apiServiceName: 'needsConnect',
    },
    getLinkedChannels: {
        path: '/linked-channels',
        method: 'GET',
        apiServiceName: 'getLinkedChannels',
    },
};
