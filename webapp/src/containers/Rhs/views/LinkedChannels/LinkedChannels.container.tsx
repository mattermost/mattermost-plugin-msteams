import React, {useCallback, useState} from 'react';

import {WarningCard} from 'components';
import usePluginApi from 'hooks/usePluginApi';
import {pluginApiServiceConfigs} from 'constants/apiService.constant';

export const LinkedChannels = () => {
    // TODO: Add Linked channel list
    const {makeApiRequestWithCompletionStatus} = usePluginApi();

    const connectAccount = useCallback(() => {
        makeApiRequestWithCompletionStatus(pluginApiServiceConfigs.connect.apiServiceName);
    }, []);

    return (
        <div className='p-20 d-flex flex-column overflow-y-auto'>
            <WarningCard
                onConnect={connectAccount}
            />
        </div>
    );
};
