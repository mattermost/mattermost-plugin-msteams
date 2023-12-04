import React, {useCallback} from 'react';

import {Button} from '@brightscout/mattermost-ui-library';

import {connectAccountFeatures, connectAccountMsg, connectButtonText, listTitle} from 'constants/connectAccount.constants';
import {Icon, IconName, WarningCard} from 'components';
import usePluginApi from 'hooks/usePluginApi';
import {pluginApiServiceConfigs} from 'constants/apiService.constant';

export const ConnectAccount = () => {
    const {makeApiRequestWithCompletionStatus} = usePluginApi();
    const connectAccount = useCallback(() => {
        makeApiRequestWithCompletionStatus(pluginApiServiceConfigs.connect.apiServiceName);
    }, []);

    return (
        <div className='p-24 d-flex flex-column overflow-y-auto'>
            <div className='flex-1 d-flex flex-column gap-16 items-center my-16'>
                <div className='d-flex flex-column gap-16 items-center'>
                    <Icon
                        width={218}
                        iconName='connectAccount'
                    />
                    <h2 className='text-center wt-600 my-0'>{connectAccountMsg}</h2>
                </div>
                <Button onClick={connectAccount}>{connectButtonText}</Button>
            </div>
            <hr className='w-full my-32'/>
            <div className='d-flex flex-column gap-24'>
                <h5 className='my-0 wt-600'>{listTitle}</h5>
                <ul className='my-0 px-0 d-flex flex-column gap-20'>
                    {connectAccountFeatures.map(({icon, text}) => (
                        <li
                            className='d-flex gap-16 items-start'
                            key={icon}
                        >
                            <Icon iconName={icon as IconName}/>
                            <h5 className='my-0 lh-20'>{text}</h5>
                        </li>
                    )) }
                </ul>
            </div>
        </div>
    );
};
