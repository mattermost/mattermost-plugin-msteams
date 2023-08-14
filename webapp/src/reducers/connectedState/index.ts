import {createSlice, PayloadAction} from '@reduxjs/toolkit';

const initialState: ConnectedState = {
    connected: false,
    username: '',
};

export const connectedSlice = createSlice({
    name: 'connected',
    initialState,
    reducers: {
        setConnected: (state, action: PayloadAction<ConnectedState>) => {
            state.connected = action.payload.connected;
            state.username = action.payload.username;
        },
    },
});

export const {setConnected} = connectedSlice.actions;

export default connectedSlice.reducer;
