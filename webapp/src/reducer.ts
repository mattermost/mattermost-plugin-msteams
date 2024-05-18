import {combineReducers} from 'redux';

import * as Actions from './types/actions';

const userConnected = (state = false, action: Actions.UserHasConnected | Actions.UserHasDisconnected) => {
    switch (action.type) {
    case Actions.USER_HAS_CONNECTED:
        return true;
    case Actions.USER_HAS_DISCONNECTED:
        return false;
    default:
        return state;
    }
};

const reducer = combineReducers({
    userConnected,
});

export default reducer;

export type PluginState = ReturnType<typeof reducer>;
