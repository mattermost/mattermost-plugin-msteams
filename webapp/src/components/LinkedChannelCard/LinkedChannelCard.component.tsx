import React from 'react';

import {Tooltip} from '@brightscout/mattermost-ui-library';

import {General as MMConstants} from 'mattermost-redux/constants';

import {Icon} from 'components/Icon';

import {LinkedChannelCardProps} from './LinkedChannelCard.types';

import './LinkedChannelCard.styles.scss';

export const LinkedChannelCard = ({msTeamsChannelName, msTeamsTeamName, mattermostChannelName, mattermostTeamName, mattermostChannelType}: LinkedChannelCardProps) => {
    const getData = (channelName: string, teamName: string) => {
        return (
            <>
                <Tooltip
                    placement='left'
                    text={channelName}
                >
                    <h5 className='my-0 msteams-linked-channel__entity-label'>{channelName}</h5>
                </Tooltip>
                <Tooltip
                    placement='left'
                    text={teamName}
                >
                    <h5 className='my-0 opacity-6 msteams-linked-channel__entity-label'>{teamName}</h5>
                </Tooltip>
            </>
        );
    };

    return (
        <div className='px-16 py-12 border-t-1 d-flex gap-4 msteams-linked-channel'>
            <div className='msteams-linked-channel__link-icon d-flex align-items-center flex-column justify-center'>
                <Icon iconName='link'/>
            </div>
            <div className='d-flex flex-column gap-6 msteams-linked-channel__body'>
                <div className='d-flex gap-8 align-items-center'>
                    <Icon iconName={mattermostChannelType === MMConstants.PRIVATE_CHANNEL ? 'lock' : 'globe'}/>
                    {getData(mattermostChannelName, mattermostTeamName)}
                </div>
                <div className='d-flex gap-8 align-items-center'>
                    <Icon iconName='msTeams'/>
                    {getData(msTeamsChannelName, msTeamsTeamName)}
                </div>
            </div>
        </div>
    );
};
