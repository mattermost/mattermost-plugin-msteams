import {Action, Store} from 'redux';

import {GlobalState} from 'mattermost-redux/types/store';

import {setGlobalModalState} from 'src/reducers/globalModal';

import {setConnected} from 'src/reducers/connectedState';
import {ModalIds} from 'src/constants';

export function handleConnect(store: Store<GlobalState, Action<Record<string, unknown>>>) {
    return (msg: WebsocketEventParams) => {
        const {data} = msg;
        const username = data.username;
        store.dispatch(setConnected({connected: true, username}) as Action);
    };
}

export function handleDisconnect(store: Store<GlobalState, Action<Record<string, unknown>>>) {
    return (_: WebsocketEventParams) => {
        store.dispatch(setConnected({connected: false, username: ''}) as Action);
    };
}

export function handleOpenLinkChannelsModal(store: Store<GlobalState, Action<Record<string, unknown>>>) {
    return (_: WebsocketEventParams) => {
        store.dispatch(setGlobalModalState({modalId: ModalIds.LINK_CHANNELS}) as Action);
    };
}
