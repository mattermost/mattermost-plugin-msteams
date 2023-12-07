import React from 'react';

import {Button} from '@brightscout/mattermost-ui-library';

import {Icon} from 'components/Icon';
import Constants from 'constants/connectAccount.constants';

import {WarningCardProps} from './WarningCard.types';

export const WarningCard = ({onConnect}: WarningCardProps) => {
    return (
        <div
            className='rhs-connect p-16'

            // TODO: Update to use util classes
            style={{
                borderRadius: '4px',
                background: '#FBF0F0',
                border: '1px solid #F7434329',
            }}
        >
            <div className='d-flex gap-12'>
                <Icon iconName='warning'/>
                <div>
                    <div className='d-flex align-items-start justify-between'>
                        <h5 className='wt-600 mt-0'>{'Please Connect your MS Teams account.'}</h5>
                    </div>
                    <p>{'You are not connected to your MS Teams account yet, please connect to your account to continue using MS Teams sync.'}
                    </p>
                    <div>
                        <Button
                            onClick={onConnect}
                        >
                            {Constants.connectButtonText}
                        </Button>
                    </div>
                </div>
            </div>
        </div>
    );
};
