type PluginState = RootState<{ [x: string]: QueryDefinition<void, BaseQueryFn<string | FetchArgs, unknown, FetchBaseQueryError, {}, FetchBaseQueryMeta>, never, void, 'msTeamsPluginApi'>; }, never, 'msTeamsPluginApi'>

type ReduxState = {
    'plugins-com.mattermost.msteams-sync': PluginState
}

type ApiRequestCompletionState = {
    requests: ApiServiceName[]
}

type ConnectedState = {
    connected: boolean;
};
