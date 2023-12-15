import {ApiRequestCompletionState, ConnectedState, PluginReduxState, SnackbarState} from 'types/common/store.d';

export const getApiRequestCompletionState = (state: PluginReduxState): ApiRequestCompletionState => state.apiRequestCompletionSlice;

export const getConnectedState = (state: PluginReduxState): ConnectedState => state.connectedStateSlice;

export const getSnackbarState = (state: PluginReduxState): SnackbarState => state.snackbarSlice;

export const getIsRhsLoading = (state: PluginReduxState): {isRhsLoading: boolean} => state.rhsLoadingSlice;
