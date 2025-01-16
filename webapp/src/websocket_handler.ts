// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

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

