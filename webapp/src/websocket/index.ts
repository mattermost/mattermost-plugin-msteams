import {Action, Store} from 'redux';

import {GlobalState} from 'mattermost-redux/types/store';

import {setConnected} from '../reducers/connectedState';
import {setNeedsConnect} from 'reducers/needsConnectState';

export function handleConnect(store: Store<GlobalState, Action<Record<string, unknown>>>) {
    return (msg: WebsocketEventParams) => {
        const {data} = msg;
        const username = data.username;
        const msteamsUserId = data.msteamsUserId;
        store.dispatch(setConnected({connected: true, username, msteamsUserId}) as Action);
        store.dispatch(setNeedsConnect({needsConnect: false}) as Action);
    };
}

export function handleDisconnect(store: Store<GlobalState, Action<Record<string, unknown>>>) {
    return (_: WebsocketEventParams) => {
        store.dispatch(setConnected({connected: false, username: '', msteamsUserId: ''}) as Action);
        store.dispatch(setNeedsConnect({needsConnect: true}) as Action);
    };
}
