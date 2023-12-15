import React from 'react';

import {render, RenderResult} from '@testing-library/react';

import {RhsTitle} from './RhsTitle.component';

let tree: RenderResult;

describe('Rhs Title component', () => {
    beforeEach(() => {
        tree = render(<RhsTitle/>);
    });

    it('Should render correctly', () => {
        expect(tree).toMatchSnapshot();
    });
});
