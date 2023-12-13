export const mockTestState = {
    'plugins-com.mattermost.msteams-sync': {
        apiRequestCompletionSlice: {
            requests: [],
        },
        webSocketEventSlice: {
            isExportZipCreated: {},
            pollStatus: {},
            isPostAdded: false,
        },
        removeModalSlice: {
            visibility: false,
            args: [],
        },
        selectSearchPostsSlice: {
            allSelected: false,
            selectedIds: {},
        },
        selectExportPostsSlice: {
            allSelected: false,
            selectedIds: {},
        },
        selectPinnedPostsSlice: {
            allSelected: false,
            selectedIds: {},
        },
        errorDialogSlice: {
            visibility: false,
            title: '',
            description: '',
        },
        confirmationDialogSlice: {
            visibility: false,
        },
        activeTabKeySlice: {
            activeTabKey: 1,
            isPollStarted: false,
        },
        msTeamsPluginApi: {
            queries: {},
            mutations: {},
            provided: {},
            subscriptions: {},
            config: {
                online: true,
                focused: true,
                middlewareRegistered: false,
                refetchOnFocus: false,
                refetchOnReconnect: false,
                refetchOnMountOrArgChange: false,
                keepUnusedDataFor: 60,
                reducerPath: 'msTeamsPluginApi',
            },
        },
    },
};
