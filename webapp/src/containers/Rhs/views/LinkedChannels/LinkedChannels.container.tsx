import React, {useCallback, useState} from 'react';

import {WarningCard} from 'components';
import usePluginApi from 'hooks/usePluginApi';
import {pluginApiServiceConfigs} from 'constants/apiService.constant';

export const LinkedChannels = () => {
    const {makeApiRequestWithCompletionStatus} = usePluginApi();
    const [isOpen, setIsOpen] = useState<boolean>(true);

    const connectAccount = useCallback(() => {
        makeApiRequestWithCompletionStatus(pluginApiServiceConfigs.connect.apiServiceName);
    }, []);

    return (
        <div className='p-20 d-flex flex-column overflow-y-auto'>
            {isOpen && (
                <WarningCard
                    onClose={() => setIsOpen(false)}
                    onConnect={connectAccount}
                />
            )}
        </div>
    );
};
