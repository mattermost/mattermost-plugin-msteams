import {Action} from 'redux';

import reducer, {resetApiRequestCompletionState, setApiRequestCompletionState} from 'reducers/apiRequest';

import {ApiRequestCompletionState} from 'types/common/store.d';

const initialState: ApiRequestCompletionState = {
    requests: [],
};

describe('API request completion reducer', () => {
    it('Should return the initial state', () => {
        expect(reducer(initialState, {} as Action)).toEqual(initialState);
    });

    it('Should handle `setApiRequestCompletionState`', () => {
        const expectedState: ApiRequestCompletionState = {requests: ['getLinkedChannels']};

        expect(reducer(initialState, setApiRequestCompletionState('getLinkedChannels'))).toEqual(expectedState);
    });

    it('Should handle `resetApiRequestCompletionState`', () => {
        const expectedState: ApiRequestCompletionState = {requests: ['connect', 'getLinkedChannels']};

        expect(reducer({requests: ['connect', 'getLinkedChannels', 'disconnectUser']}, resetApiRequestCompletionState('disconnectUser'))).toEqual(expectedState);
    });
});
