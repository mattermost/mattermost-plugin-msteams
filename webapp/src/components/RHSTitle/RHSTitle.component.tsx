import React from 'react';

import {iconUrl} from 'constants/illustrations.constants';
import {pluginTitle} from 'constants/common.constants';

export const RHSTitle = () => (
    <span className='d-flex gap-8 items-center'>
        <img
            width={24}
            height={24}
            src={iconUrl}
        />
        {pluginTitle}
    </span>
);
