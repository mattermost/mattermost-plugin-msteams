import React from 'react';

import {IconProps} from './Icon.types';
import {IconMap} from './Icon.map';

export const Icon = ({iconName, height, width, className}: IconProps) => (
    <span
        className={className}
        style={{
            width,
            height,

            // To make span fit svg element
            lineHeight: 0,
        }}
    >
        {IconMap[iconName]}
    </span>
);
