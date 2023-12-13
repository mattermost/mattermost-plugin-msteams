import React from 'react';

import {RenderResult, render} from '@testing-library/react';
import userEvent from '@testing-library/user-event';

import {getConnectedState} from 'selectors';

import {mockDispatch} from 'tests/setup';

import {mockTestState} from 'tests/mockState';

import {ReduxState} from 'types/common/store.d';

import {ConnectedAccount} from './ConnectedAccount.container';

let tree: RenderResult;

describe('Connected Account View', () => {
    beforeEach(() => {
        tree = render(<ConnectedAccount/>);
    });

    it('Should render correctly', () => {
        expect(tree).toMatchSnapshot();
    });

    it('Should render disconnect account button', () => {
        const disconnectButton = tree.getByText('Disconnect');
        expect(disconnectButton).toBeVisible();
    });

    it('Should display the name of connected user correctly', () => {
        const usernameContainer = tree.getByText('Connected as');
        const username = tree.getByText(getConnectedState(mockTestState as ReduxState).username);

        expect(usernameContainer).toContainElement(username);
        expect(username).toBeVisible();
    });

    it('Should dispatch action on clicking disconnect', async () => {
        const disconnectButton = tree.getByText('Disconnect');

        await userEvent.click(disconnectButton);
        expect(mockDispatch).toHaveBeenCalledTimes(1);
    });
});
