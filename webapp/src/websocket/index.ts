import {Action, Store} from 'redux';

import {GlobalState} from 'mattermost-redux/types/store';

import {setConnected} from 'reducers/connectedState';

export function handleConnect(store: Store<GlobalState, Action<Record<string, unknown>>>) {
    return (msg: WebsocketEventParams) => {
        const {data} = msg;
        const {username, msteamsUserId} = data;
        store.dispatch(setConnected({connected: true, username, msteamsUserId, isAlreadyConnected: false}) as Action);
    };
}

export function handleDisconnect(store: Store<GlobalState, Action<Record<string, unknown>>>) {
    return (_: WebsocketEventParams) => {
        store.dispatch(setConnected({connected: false, username: '', msteamsUserId: '', isAlreadyConnected: false}) as Action);
    };
}
