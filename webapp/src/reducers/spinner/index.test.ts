import {Action} from 'redux';

import reducer, {setIsRhsLoading} from 'reducers/spinner';

const initialState: {isRhsLoading: boolean} = {
    isRhsLoading: false,
};

describe('Spinner state reducer', () => {
    it('Should return the initial state', () => {
        expect(reducer(initialState, {} as Action)).toEqual(initialState);
    });

    it('Should handle `setIsRhsLoading`', () => {
        const expectedState: {isRhsLoading: boolean} = {isRhsLoading: true};

        expect(reducer(initialState, setIsRhsLoading(true))).toEqual(expectedState);
    });
});
