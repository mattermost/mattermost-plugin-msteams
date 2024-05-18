import {Dispatch} from 'redux';

import {userHasConnected, userHasDisconnected} from 'actions';

export const userHasConnectedWsHandler = (dispatch: Dispatch) => {
    return () => {
        dispatch(userHasConnected());
    };
};

export const userHasDisconnectedWsHandler = (dispatch: Dispatch) => {
    return () => {
        dispatch(userHasDisconnected());
    };
};

