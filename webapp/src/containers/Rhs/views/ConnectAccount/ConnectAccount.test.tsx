import React from 'react';

import {render} from '@testing-library/react';

import {ConnectAccount} from './ConnectAccount.container';

describe('Connect Account View', () => {
    it('Should render correctly', () => {
        const tree = render(<ConnectAccount/>);
        expect(tree).toMatchSnapshot();
    });
});
