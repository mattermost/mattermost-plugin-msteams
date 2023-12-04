import {createSlice, PayloadAction} from '@reduxjs/toolkit';

import {ConnectedState} from 'types/common/store.d';

const initialState: ConnectedState = {
    connected: false,
    username: '',
};

export const connectedStateSlice = createSlice({
    name: 'connectedStateSlice',
    initialState,
    reducers: {
        setConnected: (state: ConnectedState, action: PayloadAction<ConnectedState>) => {
            state.connected = action.payload.connected;
            state.username = action.payload.username;
        },
    },
});

export const {setConnected} = connectedStateSlice.actions;

export default connectedStateSlice.reducer;
