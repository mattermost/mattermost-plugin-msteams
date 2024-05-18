import * as Actions from './types/actions';

export const userHasConnected = (): Actions.UserHasConnected => ({
    type: Actions.USER_HAS_CONNECTED,
});

export const userHasDisconnected = (): Actions.UserHasDisconnected => ({
    type: Actions.USER_HAS_DISCONNECTED,
});
