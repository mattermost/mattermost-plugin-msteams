import {BaseQueryFn, FetchArgs, FetchBaseQueryError, FetchBaseQueryMeta} from '@reduxjs/toolkit/dist/query';

import {GlobalState} from 'mattermost-redux/types/store';

import {DialogProps, ListItemType} from '@brightscout/mattermost-ui-library';
import {ModalProps} from '@brightscout/mattermost-ui-library/build/components/Modal';

import {SnackbarColor} from 'components/Snackbar/Snackbar.types';
import {IconName} from 'components';

type PluginReduxState = RootState<{ [x: string]: QueryDefinition<void, BaseQueryFn<string | FetchArgs, unknown, FetchBaseQueryError, {}, FetchBaseQueryMeta>, never, void, 'msTeamsPluginApi'>; }, never, 'msTeamsPluginApi'>

interface ReduxState extends GlobalState {
    'plugins-com.mattermost.msteams-sync': PluginReduxState
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

type ModalState = {
    show?: boolean;
    isLoading?: boolean;
    mmChannel: string;
    msTeam: string;
    msChannel: string;
}

type RefetchState = {
    refetch: boolean;
};
