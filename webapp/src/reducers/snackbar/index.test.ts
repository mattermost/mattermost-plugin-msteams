import {Action} from 'redux';

import reducer, {showAlert, closeAlert} from 'reducers/snackbar';

import {SnackbarState} from 'types/common/store.d';

const initialState: SnackbarState = {
    isOpen: false,
    severity: 'default',
    message: '',
};

describe('Snackbar state reducer', () => {
    it('Should return the initial state', () => {
        expect(reducer(initialState, {} as Action)).toEqual(initialState);
    });

    it('Should handle `showAlert`', () => {
        const expectedState: SnackbarState = {isOpen: true, message: 'Custom Message', severity: 'error'};

        expect(reducer(initialState, showAlert({
            message: 'Custom Message',
            severity: 'error',
        }))).toEqual(expectedState);
    });

    it('Should handle `closeAlert`', () => {
        const expectedState: SnackbarState = {...initialState, isOpen: false};

        expect(reducer({...initialState, isOpen: true}, closeAlert())).toEqual(expectedState);
    });
});

