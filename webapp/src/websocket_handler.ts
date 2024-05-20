import {Dispatch} from 'redux';

import {userConnected, userDisconnected} from 'actions';

export const userConnectedWsHandler = (dispatch: Dispatch) => {
    return () => {
        dispatch(userConnected());
    };
};

export const userDisconnectedWsHandler = (dispatch: Dispatch) => {
    return () => {
        dispatch(userDisconnected());
    };
};

