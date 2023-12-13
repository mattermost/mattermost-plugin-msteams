import {ReduxState} from 'types/common/store.d';

export const mockTestState: Pick<ReduxState, 'plugins-com.mattermost.msteams-sync'> = {
    'plugins-com.mattermost.msteams-sync': {

        apiRequestCompletionSlice: {
            requests: [
                'whitelistUser',
                'getLinkedChannels',
            ],
        },
        connectedStateSlice: {
            connected: true,
            isAlreadyConnected: true,
            username: 'John Doe',
            msteamsUserId: 'mock_msteams_user_id',
        },
        snackbarSlice: {
            message: 'mockMessage',
            severity: 'error',
            isOpen: true,
        },
        dialogSlice: {
            description: '',
            destructive: false,
            show: true,
            primaryButtonText: '',
            isLoading: true,
            title: 'mockTitle',
        },
        rhsLoadingSlice: {
            isRhsLoading: false,
        },
        msTeamsPluginApi: {
            queries: {
                'whitelistUser(undefined)': {
                    status: 'fulfilled',
                    endpointName: 'whitelistUser',
                    requestId: 'mock_request_id',
                    startedTimeStamp: 1702107980963,
                    data: {
                        presentInWhitelist: true,
                    },
                    fulfilledTimeStamp: 1702107981697,
                },
                'needsConnect(undefined)': {
                    status: 'fulfilled',
                    endpointName: 'needsConnect',
                    requestId: 'mock_request_id',
                    startedTimeStamp: 1702107980970,
                    data: {
                        canSkip: false,
                        connected: true,
                        msteamsUserId: 'mock_msteams_user_id',
                        needsConnect: false,
                        username: 'John Doe',
                    },
                    fulfilledTimeStamp: 1702107985184,
                },
                'getLinkedChannels({"page":0,"per_page":20})': {
                    status: 'fulfilled',
                    endpointName: 'getLinkedChannels',
                    requestId: 'mock_request_id',
                    originalArgs: {
                        page: 0,
                        per_page: 20,
                    },
                    startedTimeStamp: 1702107980968,
                    data: [],
                    fulfilledTimeStamp: 1702107981716,
                },
            },
            mutations: {},
            provided: {},
            subscriptions: {
                'whitelistUser(undefined)': {
                    '4S522qve32e23e2jH78ABYk_r8nGc': {},
                },
                'needsConnect(undefined)': {
                    ghQe6Cf4HXIdPg23423a1D47SD: {},
                    'cS3T_I323233r2F0VW4kEJclgd-IV': {},
                },
                'getLinkedChannels({"page":0,"per_page":20})': {
                    GXmcNCITzra132134TGNGFtjD3: {},
                },
            },
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
