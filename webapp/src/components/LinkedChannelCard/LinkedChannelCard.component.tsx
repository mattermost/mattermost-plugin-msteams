import React, {useState} from 'react';
import {useDispatch} from 'react-redux';

import {Button, Icon as UILibIcon, Tooltip} from '@brightscout/mattermost-ui-library';
import {General as MMConstants} from 'mattermost-redux/constants';

import {Icon, Dialog} from 'components';
import {pluginApiServiceConfigs} from 'constants/apiService.constant';
import usePluginApi from 'hooks/usePluginApi';
import useApiRequestCompletionState from 'hooks/useApiRequestCompletionState';
import useAlert from 'hooks/useAlert';
import {refetch} from 'reducers/refetchState';

import {LinkedChannelCardProps} from './LinkedChannelCard.types';

import './LinkedChannelCard.styles.scss';

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

export const LinkedChannelCard = ({msTeamsChannelName, msTeamsTeamName, mattermostChannelName, mattermostTeamName, mattermostChannelType, mattermostChannelID}: LinkedChannelCardProps) => {
    const [unlinkChannelParams, setUnlinkChannelParams] = useState<UnlinkChannelParams | null>(null);

    // Show unlink and retry dialog component
    const [showUnlinkDialog, setShowUnlinkDialog] = useState(false);
    const [showRetryDialog, setShowRetryDialog] = useState(false);

    const {makeApiRequestWithCompletionStatus, getApiState} = usePluginApi();
    const dispatch = useDispatch();
    const showAlert = useAlert();

    const {isLoading: isUnlinkChannelsLoading} = getApiState(pluginApiServiceConfigs.unlinkChannel.apiServiceName, unlinkChannelParams as UnlinkChannelParams);

    const unlinkChannel = () => {
        if (unlinkChannelParams?.channelId) {
            makeApiRequestWithCompletionStatus(pluginApiServiceConfigs.unlinkChannel.apiServiceName, {channelId: unlinkChannelParams.channelId});
        }
    };

    useApiRequestCompletionState({
        serviceName: pluginApiServiceConfigs.unlinkChannel.apiServiceName,
        payload: unlinkChannelParams as UnlinkChannelParams,
        handleSuccess: () => {
            setShowUnlinkDialog(false);
            dispatch(refetch());
            showAlert({message: 'Successfully unlinked channels.', severity: 'success'});
        },
        handleError: () => {
            setShowUnlinkDialog(false);
            setShowRetryDialog(true);
        },
    });

    return (
        <div className='px-16 py-12 border-t-1 d-flex gap-4 msteams-linked-channel'>
            <div className='msteams-linked-channel__link-icon d-flex align-items-center flex-column justify-center'>
                <Icon iconName='link'/>
            </div>
            <div className='d-flex flex-column gap-6 msteams-linked-channel__body'>
                <div className='d-flex gap-8 align-items-center'>
                    {mattermostChannelType === MMConstants.PRIVATE_CHANNEL ? <Icon iconName='lock'/> : <Icon iconName='globe'/>}
                    {getData(mattermostChannelName, mattermostTeamName)}
                </div>
                <div className='d-flex gap-8 align-items-center'>
                    <Icon iconName='msTeams'/>
                    {getData(msTeamsChannelName, msTeamsTeamName)}
                </div>
            </div>
            <Button
                variant='text'
                aria-label='unlink channel'
                className='msteams-linked-channel__unlink-icon'
                onClick={() => {
                    setUnlinkChannelParams({channelId: mattermostChannelID});
                    setShowUnlinkDialog(true);
                }}
            >
                <UILibIcon
                    name='Unlink'
                    size={16}
                />
            </Button>
            <Dialog
                show={showUnlinkDialog}
                title='Unlink channels'
                destructive={true}
                primaryButtonText='Unlink channels'
                secondaryButtonText='Cancel'
                onSubmitHandler={unlinkChannel}
                onCloseHandler={() => setShowUnlinkDialog(false)}
                isLoading={isUnlinkChannelsLoading}
            >
                <>{'Are you sure you want to unlink the '}<b>{mattermostChannelName}</b>{' and '} <b>{msTeamsChannelName}</b>{' channels? Messages will no longer be synced.'}</>
            </Dialog>
            <Dialog
                show={showRetryDialog}
                title='Unlink error'
                destructive={true}
                primaryButtonText='Try Again'
                secondaryButtonText='Cancel'
                onSubmitHandler={unlinkChannel}
                onCloseHandler={() => setShowRetryDialog(false)}
                isLoading={isUnlinkChannelsLoading}
            >
                {'We were not able to unlink the selected channels. Please try again.'}
            </Dialog>
        </div>
    );
};
