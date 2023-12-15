import {Action, Store} from 'redux';

import {GlobalState} from 'mattermost-redux/types/store';

import {setConnected} from 'reducers/connectedState';
import {showLinkModal} from 'reducers/linkModal';
import {refetch} from 'reducers/refetchState';

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

export function handleUnlinkChannels(store: Store<GlobalState, Action<Record<string, unknown>>>) {
    return (_: WebsocketEventParams) => {
        store.dispatch(refetch() as Action);
    };
}

export function handleModalLink(store: Store<GlobalState, Action<Record<string, unknown>>>) {
    return (_: WebsocketEventParams) => {
        store.dispatch(showLinkModal() as Action);
    };
}

export function handleLink(store: Store<GlobalState, Action<Record<string, unknown>>>) {
    return (_: WebsocketEventParams) => {
        store.dispatch(refetch() as Action);
    };
}
