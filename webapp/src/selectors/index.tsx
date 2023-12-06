import {ApiRequestCompletionState, ConnectedState, NeedsConnectState, ReduxState, SnackbarState} from 'types/common/store.d';

const getPluginState = (state: ReduxState) => state['plugins-com.mattermost.msteams-sync'];

export const getApiRequestCompletionState = (state: ReduxState): ApiRequestCompletionState => getPluginState(state).apiRequestCompletionSlice;

export const getConnectedState = (state: ReduxState): ConnectedState => getPluginState(state).connectedStateSlice;

export const getSnackbarState = (state: ReduxState): SnackbarState => getPluginState(state).snackbarSlice;

export const getNeedsConnectedState = (state: ReduxState): NeedsConnectState => getPluginState(state).needsConnectStateSlice;
