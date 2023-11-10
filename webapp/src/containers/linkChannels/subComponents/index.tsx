import React, {useCallback, useEffect, useState} from 'react';

import {Modal} from '@brightscout/mattermost-ui-library';

import MattermostTeamPanel from './mattermostTeamPanel';
import MattermostChannelPanel from './mattermostChannelPanel';
import MicrosoftTeamPanel from './microsoftTeamPanel';
import MicrosoftChannelPanel from './microsoftChannelPanel';
import SummaryPanel from './summaryPanel';
import ResultPanel from './resultPanel';
import useApiRequestCompletionState from 'src/hooks/useApiRequestCompletionState';
import usePluginApi from 'src/hooks/usePluginApi';
import Constants from 'src/constants';

import './styles.scss';

type LinkChannelsProps = {
    open: boolean;
    close: () => void;
}

enum PanelStates {
    MM_TEAM_PANEL = 'mm_team',
    MM_CHANNEL_PANEL = 'mm_channel',
    MS_TEAM_PANEL = 'ms_team',
    MS_CHANNEL_PANEL = 'ms_channel',
    SUMMARY_PANEL = 'summary',
    RESULT_PANEL = 'result'
}

export type Team = {
    id: string,
    displayName: string,
}

export type Channel = Team & {
    type?: 'public' | 'private';
}

const LinkChannelsModal = ({open, close}: LinkChannelsProps) => {
    const [mmTeam, setMMTeam] = useState<Team | null>(null);
    const [mmChannel, setMMChannel] = useState<Channel | null>(null);
    const [msTeam, setMSTeam] = useState<Team | null>(null);
    const [msChannel, setMSChannel] = useState<Channel | null>(null);
    const {makeApiRequestWithCompletionStatus, getApiState} = usePluginApi();
    const [linkChannelsPayload, setLinkChannelsPayload] = useState<LinkChannelsPayload | null>(null);

    // Opened panel state
    const [currentPanel, setCurrentPanel] = useState(PanelStates.MM_TEAM_PANEL);

    const [apiError, setAPIError] = useState<string | null>(null);

    // Reset field states
    const resetFieldStates = useCallback(() => {
        setMMTeam(null);
        setMMChannel(null);

        setMSTeam(null);
        setMSChannel(null);
    }, []);

    // Reset panel states
    const resetPanelStates = useCallback(() => {
        setCurrentPanel(PanelStates.MM_TEAM_PANEL);
    }, []);

    const handleClose = () => {
        resetFieldStates();
        resetPanelStates();
        close();
    };

    const getTitle = (): string => {
        switch (currentPanel) {
        case PanelStates.MM_TEAM_PANEL:
            return 'Select a Mattermost Team';
        case PanelStates.MM_CHANNEL_PANEL:
            return 'Select a Channel in Mattermost Team';
        case PanelStates.MS_TEAM_PANEL:
            return 'Select a Microsoft Team';
        case PanelStates.MS_CHANNEL_PANEL:
            return 'Select a Channel in Microsoft Team';
        case PanelStates.SUMMARY_PANEL:
            return 'Channel Link Summary';
        case PanelStates.RESULT_PANEL:
            return apiError ? 'Error' : 'Success'
        default:
            return '';
        }
    };

    const getSubTitle = (): string => {
        switch (currentPanel) {
        case PanelStates.MM_TEAM_PANEL:
            return 'Select the Mattermost Team that contains the channel you are going to link';
        case PanelStates.MM_CHANNEL_PANEL:
            return 'Select the Mattermost channel you are going to link';
        case PanelStates.MS_TEAM_PANEL:
            return 'Select the Microsoft Teams team that contains the channel you are going to link';
        case PanelStates.MS_CHANNEL_PANEL:
            return 'Select the Microsoft Teams channel you are going to link';
        case PanelStates.SUMMARY_PANEL:
            return 'Review and confirm that you would like to link the channels below';
        case PanelStates.RESULT_PANEL:
            return '';
        default:
            return '';
        }
    };

    const handleNext = (): void => {
        switch (currentPanel) {
        case PanelStates.MM_TEAM_PANEL:
            setCurrentPanel(PanelStates.MM_CHANNEL_PANEL);
            break;
        case PanelStates.MM_CHANNEL_PANEL:
            setCurrentPanel(PanelStates.MS_TEAM_PANEL);
            break;
        case PanelStates.MS_TEAM_PANEL:
            setCurrentPanel(PanelStates.MS_CHANNEL_PANEL);
            break;
        case PanelStates.MS_CHANNEL_PANEL:
            setCurrentPanel(PanelStates.SUMMARY_PANEL);
            break;
        case PanelStates.SUMMARY_PANEL:
            linkChannels();
            break;
        case PanelStates.RESULT_PANEL:
            if (apiError) {
                setCurrentPanel(PanelStates.MM_TEAM_PANEL);
            } else {
                handleClose();
            }
            break;
        default:
            handleClose();
        }
    };

    const handleBack = (): void => {
        switch (currentPanel) {
        case PanelStates.MM_TEAM_PANEL:
            handleClose();
            break;
        case PanelStates.MM_CHANNEL_PANEL:
            setCurrentPanel(PanelStates.MM_TEAM_PANEL);
            break;
        case PanelStates.MS_TEAM_PANEL:
            setCurrentPanel(PanelStates.MM_CHANNEL_PANEL);
            break;
        case PanelStates.MS_CHANNEL_PANEL:
            setCurrentPanel(PanelStates.MS_TEAM_PANEL);
            break;
        case PanelStates.SUMMARY_PANEL:
            setCurrentPanel(PanelStates.MS_CHANNEL_PANEL);
            break;
        default:
            handleClose();
        }
    };

    const getPrimaryButtonText = () => {
        switch (currentPanel) {
        case PanelStates.SUMMARY_PANEL:
            return 'Link Channels';
        case PanelStates.RESULT_PANEL:
            return apiError ? 'Try Again' : 'Close';
        default:
            return 'Next';
        }
    };

    const getSecondaryButtonText = () => {
        switch (currentPanel) {
        case PanelStates.MM_TEAM_PANEL:
            return '';
        case PanelStates.RESULT_PANEL:
            return '';
        default:
            return 'Back';
        }
    };

    const linkChannels = () => {
        const payload: LinkChannelsPayload = {
            mattermostTeamID: mmTeam?.id || '',
            mattermostChannelID: mmChannel?.id || '',
            msTeamsTeamID: msTeam?.id || '',
            msTeamsChannelID: msChannel?.id || '',
        }
        setLinkChannelsPayload(payload);
        makeApiRequestWithCompletionStatus(Constants.pluginApiServiceConfigs.linkChannels.apiServiceName, payload);
    }

    const {isLoading: linkChannelsLoading} = getApiState(Constants.pluginApiServiceConfigs.linkChannels.apiServiceName, linkChannelsPayload as LinkChannelsPayload);
    useApiRequestCompletionState({
        serviceName: Constants.pluginApiServiceConfigs.linkChannels.apiServiceName,
        payload: linkChannelsPayload as LinkChannelsPayload,
        handleSuccess: () => {
            setAPIError(null);
            setCurrentPanel(PanelStates.RESULT_PANEL);
        },
        handleError: (error) => {
            setAPIError(error.data);
        }
    });


    const showLoader = linkChannelsLoading;
    return (
        <Modal
            show={open}
            onFooterCloseHandler={handleBack}
            onHeaderCloseHandler={handleClose}
            onSubmitHandler={handleNext}
            title={getTitle()}
            subtitle={getSubTitle()}
            className='teams-modal link-channels-modal wizard'
            primaryActionText={getPrimaryButtonText()}
            secondaryActionText={getSecondaryButtonText()}
            isPrimaryButtonDisabled={showLoader}
        >
            <>
                <MattermostTeamPanel
                    className={`
                        modal__body mm-team-panel wizard__primary-panel
                        ${currentPanel !== PanelStates.MM_TEAM_PANEL && 'wizard__primary-panel--fade-out'}
                    `}
                    setTeam={setMMTeam}
                    placeholder='Search Teams'
                />
                <MattermostChannelPanel
                    className={`
                        ${currentPanel === PanelStates.MM_CHANNEL_PANEL && 'wizard__secondary-panel--slide-in'}
                        ${(currentPanel !== PanelStates.MM_CHANNEL_PANEL) && 'wizard__secondary-panel--fade-out'}
                    `}
                    setChannel={setMMChannel}
                    placeholder='Search Channels'
                    teamId={mmTeam && mmTeam.id}
                />
                <MicrosoftTeamPanel
                    className={`
                        ${currentPanel === PanelStates.MS_TEAM_PANEL && 'wizard__secondary-panel--slide-in'}
                        ${(currentPanel !== PanelStates.MS_TEAM_PANEL) && 'wizard__secondary-panel--fade-out'}
                    `}
                    setTeam={setMSTeam}
                    placeholder='Search Teams'
                />
                <MicrosoftChannelPanel
                    className={`
                        ${currentPanel === PanelStates.MS_CHANNEL_PANEL && 'wizard__secondary-panel--slide-in'}
                        ${(currentPanel !== PanelStates.MS_CHANNEL_PANEL) && 'wizard__secondary-panel--fade-out'}
                    `}
                    setChannel={setMSChannel}
                    placeholder='Search Channels'
                    teamId={msTeam && msTeam.id}
                />
                <SummaryPanel
                    mmTeam={mmTeam?.displayName as string}
                    mmChannel={mmChannel?.displayName as string}
                    msTeam={msTeam?.displayName as string}
                    msChannel={msChannel?.displayName as string}
                    className={`
                        ${currentPanel === PanelStates.SUMMARY_PANEL && 'wizard__secondary-panel--slide-in'}
                        ${(currentPanel !== PanelStates.SUMMARY_PANEL) && 'wizard__secondary-panel--fade-out'}
                    `}
                />
                <ResultPanel
                    errorMessage={apiError as string}
                    className={`
                        ${currentPanel === PanelStates.RESULT_PANEL && 'wizard__secondary-panel--slide-in'}
                        ${(currentPanel !== PanelStates.RESULT_PANEL) && 'wizard__secondary-panel--fade-out'}
                    `}
                />
            </>
        </Modal>
    );
};

export default LinkChannelsModal;
