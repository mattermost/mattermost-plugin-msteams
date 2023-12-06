import {createSlice, PayloadAction} from '@reduxjs/toolkit';

import {NeedsConnectState} from 'types/common/store.d';

const initialState: NeedsConnectState = {
    needsConnect: false,
};

export const needsConnectStateSlice = createSlice({
    name: 'needsConnectStateSlice',
    initialState,
    reducers: {
        setNeedsConnect: (state: NeedsConnectState, action: PayloadAction<NeedsConnectState>) => {
            state.needsConnect = action.payload.needsConnect;
        },
    },
});

export const {setNeedsConnect} = needsConnectStateSlice.actions;

export default needsConnectStateSlice.reducer;
