import {PayloadAction, createSlice} from '@reduxjs/toolkit';

import {DialogState} from 'types/common/store.d';

const initialState: DialogState = {
    description: '',
    destructive: false,
    show: false,
    primaryButtonText: '',
    isLoading: false,
    title: '',
};

export const dialogSlice = createSlice({
    name: 'dialogSlice',
    initialState,
    reducers: {
        showDialog: (state, {payload}: PayloadAction<DialogState>) => {
            state.show = true;
            state.description = payload.description;
            state.destructive = payload.destructive;
            state.isLoading = payload.isLoading;
            state.primaryButtonText = payload.primaryButtonText;
            state.title = payload.title;
        },
        closeDialog: (state) => {
            state.show = false;
        },

    },
});

export const {showDialog, closeDialog} = dialogSlice.actions;

export default dialogSlice.reducer;
