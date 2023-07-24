import {createSlice, PayloadAction} from '@reduxjs/toolkit';

const initialState: ConnectedState = {
    connected: false,
};

export const connectedSlice = createSlice({
    name: 'connected',
    initialState,
    reducers: {
        setConnected: (state, action: PayloadAction<boolean>) => {
            state.connected = action.payload;
        },
    },
});

export const {setConnected} = connectedSlice.actions;

export default connectedSlice.reducer;
