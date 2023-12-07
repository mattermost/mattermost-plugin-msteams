import {PayloadAction, createSlice} from '@reduxjs/toolkit';

const initialState: {isRhsLoading: boolean} = {
    isRhsLoading: false,
};

export const rhsLoadingSlice = createSlice({
    name: 'rhsLoadingSlice',
    initialState,
    reducers: {
        setIsRhsLoading: (state, {payload}: PayloadAction<boolean>) => {
            state.isRhsLoading = payload;
        },
    },
});

export const {setIsRhsLoading} = rhsLoadingSlice.actions;

export default rhsLoadingSlice.reducer;
