import React from 'react';

import {render, RenderResult} from '@testing-library/react';
import userEvent from '@testing-library/user-event';

import {Dialog} from './Dialog.component';

const onCloseHandler = jest.fn();
const onSubmitHandler = jest.fn();

const dialogProps = {
    show: true,
    destructive: true,
    primaryButtonText: 'Try Again',
    secondaryButtonText: 'Cancel',
    onCloseHandler,
    onSubmitHandler,
    title: 'Unlink Channel',
};

let tree: RenderResult;

describe('Dialog component', () => {
    beforeEach(() => {
        tree = render(<Dialog {...dialogProps}>{'Are you sure you want to disconnect your Microsoft Teams Account?'}</Dialog>);
    });

    it('Should render correctly', () => {
        expect(tree).toMatchSnapshot();
    });

    it('Should close the dialog on clicking close button', () => {
        userEvent.click(tree.getByText('Cancel'));

        expect(onCloseHandler).toHaveBeenCalledTimes(1);
    });
});
