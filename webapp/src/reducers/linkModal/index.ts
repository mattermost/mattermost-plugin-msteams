import {PayloadAction, createSlice} from '@reduxjs/toolkit';

import {ModalState} from 'types/common/store.d';

const initialState: ModalState = {
    show: false,
    isLoading: false,
    mmChannel: '',
    msChannel: '',
    msTeam: '',
};

export const linkModalSlice = createSlice({
    name: 'linkModalSlice',
    initialState,
    reducers: {
        showLinkModal: (state) => {
            state.show = true;
        },
        hideLinkModal: (state) => {
            state.show = false;
        },
        setLinkModalLoading: (state, {payload}: PayloadAction<boolean>) => {
            state.isLoading = payload;
        },
        preserveState: (state, {payload}: PayloadAction<Pick<ModalState, 'mmChannel' | 'msChannel' | 'msTeam'>>) => {
            state.mmChannel = payload.mmChannel;
            state.msChannel = payload.msChannel;
            state.msTeam = payload.msTeam;
        },
        resetState: (state) => {
            state.show = false;
            state.isLoading = false;
            state.mmChannel = '';
            state.msChannel = '';
            state.msTeam = '';
        },
    },
});

export const {showLinkModal, hideLinkModal, setLinkModalLoading, preserveState, resetState} = linkModalSlice.actions;

export default linkModalSlice.reducer;
