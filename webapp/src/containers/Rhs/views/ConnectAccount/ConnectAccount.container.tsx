import React, {useCallback} from 'react';

import {Button} from '@brightscout/mattermost-ui-library';

import Constants from 'constants/connectAccount.constants';
import {Icon, IconName} from 'components';
import usePluginApi from 'hooks/usePluginApi';
import {pluginApiServiceConfigs} from 'constants/apiService.constant';
import useApiRequestCompletionState from 'hooks/useApiRequestCompletionState';
import useAlert from 'hooks/useAlert';

export const ConnectAccount = () => {
    const showAlert = useAlert();
    const {makeApiRequestWithCompletionStatus, getApiState} = usePluginApi();

    const connectAccount = useCallback(() => {
        makeApiRequestWithCompletionStatus(pluginApiServiceConfigs.connect.apiServiceName);
    }, []);

    useApiRequestCompletionState({
        serviceName: pluginApiServiceConfigs.connect.apiServiceName,
        handleError: () => {
            showAlert({message: Constants.connectAccountUnsuccessfulMsg, severity: 'error'});
        },
    });

    return (
        <div className='msteams-sync-utils'>
            <div className='p-24 d-flex flex-column overflow-y-auto'>
                <div className='flex-1 d-flex flex-column gap-16 align-items-center my-16'>
                    <div className='d-flex flex-column gap-16 align-items-center'>
                        <Icon
                            width={218}
                            iconName='connectAccount'
                        />
                        <h2 className='text-center wt-600 my-0'>{Constants.connectAccountMsg}</h2>
                    </div>
                    <Button onClick={connectAccount}>{Constants.connectButtonText}</Button>
                </div>
                <hr className='w-full my-32'/>
                <div className='d-flex flex-column gap-24'>
                    <h5 className='my-0 wt-600'>{Constants.listTitle}</h5>
                    <ul className='my-0 px-0 d-flex flex-column gap-20'>
                        {Constants.connectAccountFeatures.map(({icon, text}) => (
                            <li
                                className='d-flex gap-16 align-items-start'
                                key={icon}
                            >
                                <Icon iconName={icon as IconName}/>
                                <h5 className='my-0 lh-20'>{text}</h5>
                            </li>
                        )) }
                    </ul>
                </div>
            </div>
        </div>
    );
};
