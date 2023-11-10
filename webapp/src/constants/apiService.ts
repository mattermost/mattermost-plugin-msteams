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
    disconnectUser: {
        path: '/disconnect',
        method: 'GET',
        apiServiceName: 'disconnectUser',
    },
    unlinkChannel: {
        path: '/unlink-channels',
        method: 'DELETE',
        apiServiceName: 'unlinkChannel',
    },
    searchMSTeams: {
        path: '/msteams/teams',
        method: 'GET',
        apiServiceName: 'searchMSTeams'
    },
    searchMSChannels: {
        path: '/msteams/teams/{team_id}/channels',
        method: 'GET',
        apiServiceName: 'searchMSChannels'
    },
    linkChannels: {
        path: '/link-channels',
        method: 'POST',
        apiServiceName: 'linkChannels'
    },
};
