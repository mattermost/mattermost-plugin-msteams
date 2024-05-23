import * as Actions from './types/actions';

export const userConnected = (): Actions.UserHasConnected => ({
    type: Actions.USER_CONNECTED,
});

export const userDisconnected = (): Actions.UserHasDisconnected => ({
    type: Actions.USER_DISCONNECTED,
});
