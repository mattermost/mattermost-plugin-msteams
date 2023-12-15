import {TeamsState} from 'mattermost-redux/types/teams';

import {ApiRequestCompletionState, ConnectedState, ModalState, ReduxState, RefetchState, SnackbarState, PluginReduxState} from 'types/common/store.d';

export const getApiRequestCompletionState = (state: PluginReduxState): ApiRequestCompletionState => state.apiRequestCompletionSlice;

export const getConnectedState = (state: PluginReduxState): ConnectedState => state.connectedStateSlice;

export const getSnackbarState = (state: PluginReduxState): SnackbarState => state.snackbarSlice;

export const getLinkModalState = (state: PluginReduxState): ModalState => state.linkModalSlice;

export const getRefetchState = (state: PluginReduxState): RefetchState => state.refetchSlice;

export const getIsRhsLoading = (state: PluginReduxState): {isRhsLoading: boolean} => state.rhsLoadingSlice;
