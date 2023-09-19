import {ModalIds} from '../constants';

// const getPluginState = (state: ReduxState) => state['plugins-com.mattermost.msteams-sync'];

export const getApiRequestCompletionState = (state: PluginState): ApiRequestCompletionState => state.apiRequestCompletionSlice;

export const getGlobalModalState = (state: PluginState): GlobalModalState => state.globalModalSlice;

export const isLinkChannelsModalOpen = (state: PluginState): boolean => state.globalModalSlice?.modalId === ModalIds.LINK_CHANNELS;
