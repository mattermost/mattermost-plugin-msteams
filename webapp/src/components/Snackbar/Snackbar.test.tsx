import React from 'react';

import {render, RenderResult} from '@testing-library/react';
import userEvent from '@testing-library/user-event';

import {mockDispatch} from 'tests/setup';

import {Snackbar} from './Snackbar.component';

let tree: RenderResult;

describe('Snackbar component', () => {
    beforeEach(() => {
        tree = render(<Snackbar/>);
    });

    it('Should render correctly', () => {
        expect(tree).toMatchSnapshot();
    });

    it('Should show correct type', () => {
        expect(tree.container.firstChild?.firstChild).toHaveClass('bg-error');
    });

    it('Should show correct message', () => {
        const snackbarText = tree.getByText('mockMessage');
        expect(snackbarText).toBeVisible();
    });

    it('Should dispatch action on clicking close icon', () => {
        expect(tree.getAllByRole('button').length).toEqual(1);

        userEvent.click(tree.getAllByRole('button')[0]);
        expect(mockDispatch).toHaveBeenCalledTimes(1);
    });
});
