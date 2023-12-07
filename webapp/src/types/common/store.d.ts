import {BaseQueryFn, FetchArgs, FetchBaseQueryError, FetchBaseQueryMeta} from '@reduxjs/toolkit/dist/query';

import {GlobalState} from 'mattermost-redux/types/store';

import {SnackbarColor} from 'components/Snackbar/Snackbar.types';

type PluginState = RootState<{ [x: string]: QueryDefinition<void, BaseQueryFn<string | FetchArgs, unknown, FetchBaseQueryError, {}, FetchBaseQueryMeta>, never, void, 'msTeamsPluginApi'>; }, never, 'msTeamsPluginApi'>

interface ReduxState extends GlobalState {
    'plugins-com.mattermost.msteams-sync': PluginState
}

type ApiRequestCompletionState = {
    requests: PluginApiServiceName[]
}

type ConnectedState = {
    connected: boolean;
    username: string;
};

type SnackbarState = {
    severity: SnackbarColor;
    message: string;
    isOpen: boolean;
};

type SnackbarActionPayload = Pick<SnackbarState, 'message' | 'severity'>;
