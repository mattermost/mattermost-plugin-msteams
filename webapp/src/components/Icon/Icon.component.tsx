import React from 'react';

import {IconProps} from './Icon.types';
import {IconMap} from './Icon.map';

export const Icon = ({iconName, height, width, className}: IconProps) => (
    <span
        className={className}
        style={{
            width,
            height,
        }}
    >
        {IconMap[iconName]}
    </span>
);

