import React, {useCallback} from 'react';

import {Tooltip} from '@brightscout/mattermost-ui-library';

import {General as MMConstants} from 'mattermost-redux/constants';

import useApiRequestCompletionState from '../../hooks/useApiRequestCompletionState';
import usePluginApi from '../../hooks/usePluginApi';

import Constants from '../../constants';

import './rhs.scss';

const Rhs = (): JSX.Element => {
    const {state, makeApiRequestWithCompletionStatus, getApiState} = usePluginApi();
    const connected = state.connectedReducer.connected;

    const connectAccount = useCallback(() => {
        makeApiRequestWithCompletionStatus(Constants.pluginApiServiceConfigs.connect.apiServiceName);
    }, []);

    const {data: connectData} = getApiState(Constants.pluginApiServiceConfigs.connect.apiServiceName);

    useApiRequestCompletionState({
        serviceName: Constants.pluginApiServiceConfigs.connect.apiServiceName,
        handleSuccess: () => {
            if (connectData) {
                window.open((connectData as ConnectData).connectUrl, '_blank');
            }
        },
    });

    // TODO: remove dummy data after api integration
    // const channelLinkData: ChannelLinkData[] = [];
    const channelLinkData: ChannelLinkData[] = [
        {
            msTeamsChannelName: 'msTeamsChannelName-1',
            msTeamsTeamName: 'msTeamsTeamName-1',
            mattermostChannelName: 'mattermostChannelName-1',
            mattermostTeamName: 'mattermostTeamName-1',
            channelType: 'P',
        },
        {
            msTeamsChannelName: 'msC-2',
            msTeamsTeamName: 'msT-2',
            mattermostChannelName: 'mmC-2',
            mattermostTeamName: 'mmT-2',
            channelType: 'O',
        },
        {
            msTeamsChannelName: 'msTeamsChannelName-3',
            msTeamsTeamName: 'msTeamsTeamName-3',
            mattermostChannelName: 'mattermostChannelName-3',
            mattermostTeamName: 'mattermostTeamName-3',
            channelType: 'P',
        },
    ];

    return (
        <>
            {connected ? (

            // TODO: add disconnect feature later.
                <>{'Connected successfully'}</>
            ) : (
                <div className='msteams-sync-rhs-connect'>
                    <div className='msteams-sync-rhs-connect__heading'>
                        <img
                            className='msteams-sync-rhs-connect__icon'
                            src={Constants.notConnectIconUrl}
                        />
                        <div className='msteams-sync-rhs-connect__body'>
                            <div className='msteams-sync-rhs-connect__title'>{'Please Connect your MS Teams account.'}</div>
                            {'You are not connected to your MS Teams account yet, please connect to your account to continue using Teams sync.'}
                            <div>
                                <button
                                    className='btn btn-primary msteams-sync-rhs-connect__connect-button'
                                    onClick={connectAccount}
                                >
                                    {'Connect Account'}
                                </button>
                            </div>
                        </div>
                    </div>
                </div>

            )}
            <div className='msteams-sync-rhs-divider'/>
            <div className='msteams-sync-rhs-body'>
                <div className='msteams-sync-rhs-body__title'>{'Linked Channels'}</div>
                <div className='msteams-sync-rhs-body__subtitle'>{'Messages will be synchronized between linked channels.'}</div>
                {/* TODO: add search bar later. */}
                {channelLinkData.length ? (
                    <div className='link-data__container'>
                        <div className='link-data__title'>
                            <div className='link-data__title-values'>{'Mattermost'}</div>
                            <div className='link-data__title-values'>{'MS Team'}</div>
                        </div>
                        {channelLinkData.map((link) => (
                            <div
                                className='link-data'
                                key={link.msTeamsTeamName}
                            >
                                <div className='link-data__mm-values'>
                                    <img src={link.channelType === MMConstants.PRIVATE_CHANNEL ? Constants.mmPrivateChannelIconUrl : Constants.mmPublicChannelIconUrl}/>
                                    <div className='link-data__body'>
                                        <Tooltip
                                            text={link.mattermostChannelName}
                                        >
                                            <div className='link-data__channel-name'>
                                                {link.mattermostChannelName}
                                            </div>
                                        </Tooltip>
                                        <Tooltip text={link.mattermostTeamName}>
                                            <div className='link-data__team-name'>{link.mattermostTeamName}</div>
                                        </Tooltip>
                                    </div>
                                </div>
                                <div className='channel-link-icon'>
                                    <img src={Constants.linkIconUrl}/>
                                </div>
                                <div className='link-data__ms-values'>
                                    <img src={Constants.msteamsIconUrl}/>
                                    <div className='link-data__body'>
                                        <Tooltip text={link.msTeamsChannelName}>
                                            <div className='link-data__channel-name'>{link.msTeamsChannelName}</div>
                                        </Tooltip>
                                        <Tooltip text={link.msTeamsTeamName}>
                                            <div className='link-data__team-name'>{link.msTeamsTeamName}</div>
                                        </Tooltip>
                                    </div>
                                </div>
                                <div className='channel-unlink-icon'>
                                    <img src={Constants.channelUnlinkIconUrl}/>
                                </div>
                            </div>
                        ))}
                    </div>
                ) : (
                    <div className='no-link'>
                        <img src={Constants.globeIconUrl}/>
                        <div className='no-link__title'>{'There are no linked channels'}</div>
                        {connected && (
                            <button
                                className='btn btn-primary'

                                // TODO: Update later
                                // eslint-disable-next-line no-alert
                                onClick={() => alert('open modal!!!!!!!!!')}
                            >
                                {'Link New Channel'}
                            </button>
                        )}
                    </div>
                )}
            </div>
            {connected && channelLinkData.length && (
                <>
                    <div className='msteams-sync-rhs-divider'/>
                    <div className='link-channel-btn'>
                        <button
                            className='btn btn-primary'

                            // TODO: Update later
                            // eslint-disable-next-line no-alert
                            onClick={() => alert('open modal!!!!!!!!!')}
                        >
                            {'Link Channel'}
                        </button>
                    </div>
                </>
            )}
        </>
    );
};

export default Rhs;
