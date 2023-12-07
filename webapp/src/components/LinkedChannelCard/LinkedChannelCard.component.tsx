import React from 'react';

import {Icon} from 'components/Icon';

import {LinkedChannelCardProps} from './LinkedChannelCard.types';

import './LinkedChannelCard.styles.scss';

export const LinkedChannelCard = ({msTeamsChannelName, msTeamsTeamName, mattermostChannelName, mattermostTeamName}: LinkedChannelCardProps) => (
    <div className='px-16 py-12 border-t-1 d-flex gap-4 msteams-linked-channel'>
        <div className='msteams-linked-channel__link-icon d-flex align-items-center flex-column justify-center'>
            <Icon iconName='link'/>
        </div>
        <div className='d-flex flex-column gap-6'>
            <div className='d-flex gap-8 align-items-center'>
                {/* TODO: Update icon on basis of channel type  */}
                <Icon iconName='globe'/>
                <h5 className='my-0'>{mattermostChannelName}</h5>
                <h5 className='my-0 opacity-6'>{mattermostTeamName}</h5>
            </div>
            <div className='d-flex gap-8 align-items-center'>
                <Icon iconName='msTeams'/>
                <h5 className='my-0'>{msTeamsChannelName}</h5>
                <h5 className='my-0 opacity-6'>{msTeamsTeamName}</h5>
            </div>
        </div>
    </div>
);
