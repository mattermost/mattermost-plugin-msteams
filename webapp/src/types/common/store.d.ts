import {BaseQueryFn, FetchArgs, FetchBaseQueryError, FetchBaseQueryMeta} from '@reduxjs/toolkit/dist/query';

import {GlobalState} from 'mattermost-redux/types/store';

import {DialogProps} from '@brightscout/mattermost-ui-library';

import {SnackbarColor} from 'components/Snackbar/Snackbar.types';
import {IconName} from 'components';

type PluginState = RootState<{ [x: string]: QueryDefinition<void, BaseQueryFn<string | FetchArgs, unknown, FetchBaseQueryError, {}, FetchBaseQueryMeta>, never, void, 'msTeamsPluginApi'>; }, never, 'msTeamsPluginApi'>

interface ReduxState extends GlobalState {
    'plugins-com.mattermost.msteams-sync': PluginState
}

type ApiRequestCompletionState = {
    requests: PluginApiServiceName[]
}

type ConnectedState = {
    connected: boolean;
    isAlreadyConnected: boolean;
    username: string;
    msteamsUserId: string;
};

type CanSeeRhsState = {
    canSeeRhs: boolean;
}

type NeedsConnectState = {
    needsConnect: boolean;
};

type SnackbarState = {
    severity: SnackbarColor;
    message: string;
    isOpen: boolean;
};

type SnackbarActionPayload = Pick<SnackbarState, 'message' | 'severity'>;

type DialogState = Pick<DialogProps, 'destructive' | 'description' | 'show' | 'primaryButtonText' | 'secondaryButtonText' | 'title'> & {
    isLoading?: boolean
}
