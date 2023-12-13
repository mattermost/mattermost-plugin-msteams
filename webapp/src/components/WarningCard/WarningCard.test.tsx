import React from 'react';

import {render, RenderResult} from '@testing-library/react';
import userEvent from '@testing-library/user-event';

import {WarningCard} from './WarningCard.component';
import {WarningCardProps} from './WarningCard.types';

const onMockConnect = jest.fn();

const warningCardProps: WarningCardProps = {
    onConnect: onMockConnect,
};

let tree: RenderResult;

describe('Warning Card component', () => {
    beforeEach(() => {
        tree = render(<WarningCard {...warningCardProps}/>);
    });

    it('Should render correctly', () => {
        expect(tree).toMatchSnapshot();
    });

    it('Should call connect function on click of button', () => {
        expect(tree.getAllByRole('button').length).toEqual(1);

        userEvent.click(tree.getAllByRole('button')[0]);
        expect(onMockConnect).toHaveBeenCalledTimes(1);
    });
});
