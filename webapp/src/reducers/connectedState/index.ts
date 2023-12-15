import {createSlice, PayloadAction} from '@reduxjs/toolkit';

import {ConnectedState} from 'types/common/store.d';

const initialState: ConnectedState = {
    connected: false,
    isAlreadyConnected: false,
    username: '',
    msteamsUserId: '',
};

export const connectedStateSlice = createSlice({
    name: 'connectedStateSlice',
    initialState,
    reducers: {
        setConnected: (state: ConnectedState, action: PayloadAction<ConnectedState>) => {
            state.connected = action.payload.connected;
            state.isAlreadyConnected = action.payload.isAlreadyConnected;
            state.username = action.payload.username;
            state.msteamsUserId = action.payload.msteamsUserId;
        },
    },
});

export const {setConnected} = connectedStateSlice.actions;

export default connectedStateSlice.reducer;
