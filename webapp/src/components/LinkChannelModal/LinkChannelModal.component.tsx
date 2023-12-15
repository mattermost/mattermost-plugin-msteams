import React, {useState} from 'react';

import {LinearProgress, Modal} from '@brightscout/mattermost-ui-library';

import {useDispatch, useSelector} from 'react-redux';

import usePluginApi from 'hooks/usePluginApi';

import {getLinkModalState} from 'selectors';

import {Dialog} from 'components/Dialog';
import {pluginApiServiceConfigs} from 'constants/apiService.constant';
import useApiRequestCompletionState from 'hooks/useApiRequestCompletionState';
import {hideLinkModal, preserveState, resetState, setLinkModalLoading, showLinkModal} from 'reducers/linkModal';
import useAlert from 'hooks/useAlert';

import {refetch} from 'reducers/refetchState';

import {ReduxState} from 'types/common/store.d';

import {SearchMSChannels} from './SearchMSChannels';
import {SearchMSTeams} from './SearchMSTeams';
import {SearchMMChannels} from './SearchMMChannels';

export const LinkChannelModal = () => {
    const dispatch = useDispatch();
    const showAlert = useAlert();
    const {state, makeApiRequestWithCompletionStatus} = usePluginApi();
    const {show = false, isLoading} = getLinkModalState(state);
    const {currentTeamId} = useSelector((reduxState:ReduxState) => reduxState.entities.teams);

    // Show retry dialog component
    const [showRetryDialog, setShowRetryDialog] = useState(false);

    const [mmChannel, setMMChannel] = useState<MMTeamOrChannel | null>(null);
    const [msTeam, setMSTeam] = useState<MSTeamOrChannel | null>(null);
    const [msChannel, setMSChannel] = useState<MSTeamOrChannel | null>(null);
    const [linkChannelsPayload, setLinkChannelsPayload] = useState<LinkChannelsPayload | null>(null);

    const handleModalClose = (preserve?: boolean) => {
        if (!preserve) {
            setMMChannel(null);
            setMSTeam(null);
            setMSChannel(null);
        }
        dispatch(resetState());
        dispatch(hideLinkModal());
    };

    const handleChannelLinking = () => {
        const payload: LinkChannelsPayload = {
            mattermostTeamID: currentTeamId || '',
            mattermostChannelID: mmChannel?.id || '',
            msTeamsTeamID: msTeam?.ID || '',
            msTeamsChannelID: msChannel?.ID || '',
        };
        setLinkChannelsPayload(payload);
        makeApiRequestWithCompletionStatus(pluginApiServiceConfigs.linkChannels.apiServiceName, payload);
        dispatch(setLinkModalLoading(true));
    };

    useApiRequestCompletionState({
        serviceName: pluginApiServiceConfigs.linkChannels.apiServiceName,
        payload: linkChannelsPayload as LinkChannelsPayload,
        handleSuccess: () => {
            dispatch(setLinkModalLoading(false));
            handleModalClose();
            dispatch(refetch());
            showAlert({
                message: 'Successfully linked channels',
                severity: 'success',
            });
        },
        handleError: () => {
            dispatch(setLinkModalLoading(false));
            handleModalClose(true);
            setShowRetryDialog(true);
        },
    });

    return (
        <>
            <Modal
                show={show}
                className='msteams-sync-modal msteams-sync-utils'
                title='Link a channel'
                subtitle='Link a channel in Mattermost with a channel in Microsoft Teams'
                primaryActionText='Link Channels'
                secondaryActionText='Cancel'
                onFooterCloseHandler={handleModalClose}
                onHeaderCloseHandler={handleModalClose}
                isPrimaryButtonDisabled={!mmChannel || !msChannel || !msTeam}
                onSubmitHandler={handleChannelLinking}
                backdrop={true}
            >
                {isLoading && <LinearProgress className='fixed w-full left-0 top-100'/>}
                <SearchMMChannels
                    setChannel={setMMChannel}
                    teamId={currentTeamId}
                />
                <hr className='w-full my-32'/>
                <SearchMSTeams setMSTeam={setMSTeam}/>
                <SearchMSChannels
                    setChannel={setMSChannel}
                    teamId={msTeam?.ID}
                />
            </Modal>
            <Dialog
                show={showRetryDialog}
                destructive={true}
                primaryButtonText='Try Again'
                secondaryButtonText='Cancel'
                title='Unable to link channels'
                onSubmitHandler={() => {
                    dispatch(preserveState({
                        mmChannel: mmChannel?.displayName ?? '',
                        msChannel: msChannel?.DisplayName ?? '',
                        msTeam: msTeam?.DisplayName ?? '',
                    }));
                    setShowRetryDialog(false);
                    dispatch(showLinkModal());
                }}
                onCloseHandler={() => {
                    setShowRetryDialog(false);
                    dispatch(resetState());
                }}
            >
                {'We were not able to link the selected channels. Please try again.'}
            </Dialog>
        </>
    );
};
