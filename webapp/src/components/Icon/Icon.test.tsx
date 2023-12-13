import React from 'react';

import {render, RenderResult} from '@testing-library/react';

import {Icon} from './Icon.component';
import {IconProps} from './Icon.types';

const iconProps: IconProps = {
    iconName: 'close',
    className: 'mockClassName',
};

let tree: RenderResult;

describe('Icon component', () => {
    beforeEach(() => {
        tree = render(<Icon {...iconProps}/>);
    });

    it('Should render correctly', () => {
        expect(tree).toMatchSnapshot();
    });

    it('Should apply the passed className prop', () => {
        expect(tree.container.firstChild).toHaveClass(iconProps.className as string, {exact: true});
    });
});
