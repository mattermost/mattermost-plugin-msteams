import React from 'react';

import Constants from 'constants/index';

export const RHSTitle = () => (
    <span className='d-flex gap-8 items-center'>
        <img
            width={24}
            height={24}
            src={Constants.iconUrl}
        />
        {Constants.pluginTitle}
    </span>
);
