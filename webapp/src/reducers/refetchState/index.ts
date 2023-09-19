import {createSlice} from '@reduxjs/toolkit';

const initialState: RefetchState = {
    refetch: false,
};

export const refetchSlice = createSlice({
    name: 'refetch',
    initialState,
    reducers: {
        refetch: (state) => {
            state.refetch = true;
        },
        resetRefetch: (state) => {
            state.refetch = false;
        },
    },
});

export const {refetch, resetRefetch} = refetchSlice.actions;

export default refetchSlice.reducer;
