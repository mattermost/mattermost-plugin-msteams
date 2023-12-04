import {ApiRequestCompletionState, ConnectedState, ReduxState} from 'types/common/store.d';

const getPluginState = (state: ReduxState) => state['plugins-com.mattermost.msteams-sync'];

export const getApiRequestCompletionState = (state: ReduxState): ApiRequestCompletionState => getPluginState(state).apiRequestCompletionSlice;

export const getConnectedState = (state: ReduxState): ConnectedState => getPluginState(state).connectedStateSlice;
