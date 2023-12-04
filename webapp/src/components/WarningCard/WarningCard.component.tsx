import React from 'react';

import './WarningCard.styles.scss';
import {Button} from '@brightscout/mattermost-ui-library';

import {Icon} from 'components/Icon';
import {connectButtonText} from 'constants/connectAccount.constants';

import {WarningCardProps} from './WarningCard.types';

export const WarningCard = ({onClose, onConnect}: WarningCardProps) => {
    return (
        <div className='rhs-connect p-16'>
            <div className='d-flex gap-12'>
                <Icon iconName='warning'/>
                <div>
                    <div className='d-flex items-start justify-between'>
                        <h5 className='wt-600 mt-0'>{'Please Connect your MS Teams account.'}</h5>
                        <Button
                            variant='text'
                            className='relative bottom-4'
                            onClick={onClose}
                        >
                            <Icon iconName='close'/>
                        </Button>
                    </div>
                    <p>{'You are not connected to your MS Teams account yet, please connect to your account to continue using MS Teams sync.'}
                    </p>
                    <div>
                        <Button
                            onClick={onConnect}
                        >
                            {connectButtonText}
                        </Button>
                    </div>
                </div>

            </div>
        </div>
    );
};
