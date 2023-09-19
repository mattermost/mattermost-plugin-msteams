import {createSlice, PayloadAction} from '@reduxjs/toolkit';

const initialState: GlobalModalState = {
    modalId: null,
    data: null,
};

export const globalModalSlice = createSlice({
    name: 'globalModalSlice',
    initialState,
    reducers: {
        setGlobalModalState: (state: GlobalModalState, action: PayloadAction<GlobalModalState>) => {
            state.modalId = action.payload.modalId;
            state.data = action.payload.data;
        },
        resetGlobalModalState: (state: GlobalModalState) => {
            state.modalId = null;
            state.data = null;
        },
    },
});

export const {setGlobalModalState, resetGlobalModalState} = globalModalSlice.actions;

export default globalModalSlice.reducer;
