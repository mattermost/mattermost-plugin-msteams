import React, {useCallback, useEffect, useMemo, useState} from 'react';
import InfiniteScroll from 'react-infinite-scroll-component';

import {Spinner, Tooltip} from '@brightscout/mattermost-ui-library';

import {General as MMConstants} from 'mattermost-redux/constants';

import useApiRequestCompletionState from 'hooks/useApiRequestCompletionState';
import usePluginApi from 'hooks/usePluginApi';

import Constants from 'constants/index';
import {SVGIcons} from 'constants/icons';

import './rhs.scss';

const Rhs = (): JSX.Element => {
    const {state, makeApiRequestWithCompletionStatus, getApiState} = usePluginApi();
    const connected = state.connectedReducer.connected;

    const [totalLinkedChannels, setTotalLinkedChannels] = useState<ChannelLinkData[]>([]);
    const [paginationQueryParams, setPaginationQueryParams] = useState<PaginationQueryParams>({
        page: Constants.DefaultPage,
        per_page: Constants.DefaultPerPage,
    });
    const [getLinkedChannelsParams, setGetLinkedChannelsParams] = useState<PaginationQueryParams | null>(null);

    const connectAccount = useCallback(() => {
        makeApiRequestWithCompletionStatus(Constants.pluginApiServiceConfigs.connect.apiServiceName);
    }, []);

    useEffect(() => {
        const linkedChannelsParams: PaginationQueryParams = {page: paginationQueryParams.page, per_page: paginationQueryParams.per_page};
        setGetLinkedChannelsParams(linkedChannelsParams);
        makeApiRequestWithCompletionStatus(Constants.pluginApiServiceConfigs.getLinkedChannels.apiServiceName, linkedChannelsParams);
    }, [paginationQueryParams]);

    const {data: connectData} = getApiState(Constants.pluginApiServiceConfigs.connect.apiServiceName);
    const {data: linkedChannels, isLoading} = getApiState(Constants.pluginApiServiceConfigs.getLinkedChannels.apiServiceName, getLinkedChannelsParams as PaginationQueryParams);

    useApiRequestCompletionState({
        serviceName: Constants.pluginApiServiceConfigs.connect.apiServiceName,
        handleSuccess: () => {
            if (connectData) {
                window.open((connectData as ConnectData).connectUrl, '_blank');
            }
        },
    });

    useApiRequestCompletionState({
        serviceName: Constants.pluginApiServiceConfigs.getLinkedChannels.apiServiceName,
        payload: getLinkedChannelsParams as PaginationQueryParams,
        handleSuccess: () => {
            if (linkedChannels) {
                setTotalLinkedChannels([...totalLinkedChannels, ...(linkedChannels as ChannelLinkData[])]);
            }
        },
    });

    // Increase the page number by 1
    const handlePagination = () => {
        setPaginationQueryParams({...paginationQueryParams, page: paginationQueryParams.page + 1,
        });
    };

    const hasMoreLinkedChannels = useMemo<boolean>(() => (
        (totalLinkedChannels.length - (paginationQueryParams.page * Constants.DefaultPerPage) === Constants.DefaultPerPage)
    ), [totalLinkedChannels]);

    return (
        <div className='msteams-sync-rhs'>
            {connected ? (

            // TODO: add disconnect feature later.
                <>{'Connected successfully'}</>
            ) : (
                <div className='rhs-connect'>
                    <div className='rhs-connect__heading'>
                        <div
                            className='rhs-connect__icon'
                        >
                            {SVGIcons.notConnectIcon}
                        </div>
                        <div className='rhs-connect__body'>
                            <div className='rhs-connect__title'>{'Please Connect your MS Teams account.'}</div>
                            {'You are not connected to your MS Teams account yet, please connect to your account to continue using MS Teams sync.'}
                            <div>
                                <button
                                    className='btn btn-primary rhs-connect__connect-button'
                                    onClick={connectAccount}
                                >
                                    {'Connect Account'}
                                </button>
                            </div>
                        </div>
                    </div>
                </div>
            )}
            <div className='rhs-body-container'>
                <div className='rhs-body'>
                    <div className='rhs-body__title'>{'Linked Channels'}</div>
                    <div className='rhs-body__subtitle'>{'Messages will be synchronized between linked channels.'}</div>
                    {/* TODO: add search bar later. */}
                    {isLoading && !paginationQueryParams.page && <Spinner className='rhs-body__spinner'/>}
                    {Boolean(totalLinkedChannels.length) && (
                        <div className='link-data__container'>
                            <div className='link-data__title'>
                                <div className='link-data__title-values'>{'Mattermost'}</div>
                                <div className='link-data__title-values'>{'MS Team'}</div>
                            </div>
                            <div
                                id='scrollableArea'
                                className='link-data__container-values'
                            >
                                <InfiniteScroll
                                    dataLength={totalLinkedChannels.length}
                                    next={handlePagination}
                                    hasMore={hasMoreLinkedChannels}
                                    loader={<Spinner className='link-data__spinner'/>}
                                    endMessage={
                                        <p className='text-center'>
                                            <b>{'No more linked channels present.'}</b>
                                        </p>
                                    }
                                    scrollableTarget='scrollableArea'
                                >
                                    {totalLinkedChannels.map((link) => (
                                        <div
                                            className='link-data'
                                            key={link.msTeamsTeamName}
                                        >
                                            <div className='link-data__mm-values'>
                                                {link.mattermostChannelType === MMConstants.PRIVATE_CHANNEL ? (
                                                    <>{SVGIcons.mmPrivateChannel}</>
                                                ) : (
                                                    <>{SVGIcons.mmPublicChannel}</>
                                                )}
                                                <div className='link-data__body'>
                                                    <Tooltip text={link.mattermostChannelName}>
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
                                                {SVGIcons.linkIcon}
                                            </div>
                                            <div className='link-data__ms-values'>
                                                {SVGIcons.msTeamsIcon}
                                                <div className='link-data__body'>
                                                    <Tooltip text={link.msTeamsChannelName}>
                                                        <div className='link-data__channel-name'>{link.msTeamsChannelName}</div>
                                                    </Tooltip>
                                                    <Tooltip text={link.msTeamsTeamName}>
                                                        <div className='link-data__team-name'>{link.msTeamsTeamName}</div>
                                                    </Tooltip>
                                                </div>
                                            </div>
                                            <Tooltip text={'Unlink'}>
                                                <div
                                                    className='channel-unlink-icon'

                                                    // TODO: Update later
                                                    // eslint-disable-next-line no-alert
                                                    onClick={() => alert('Unlink chanel')}
                                                >
                                                    {SVGIcons.channelUnlink}
                                                </div>
                                            </Tooltip>
                                        </div>
                                    ))}
                                </InfiniteScroll>
                            </div>
                        </div>
                    )}
                    {!totalLinkedChannels.length && !isLoading && (
                        <div className='no-link'>
                            {SVGIcons.globeIcon}
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
            </div>
            {connected && Boolean(totalLinkedChannels.length) && (
                <div className='rhs-footer'>
                    <div className='rhs-footer__link-btn'>
                        <button
                            className='btn btn-primary'

                            // TODO: Update later
                            // eslint-disable-next-line no-alert
                            onClick={() => alert('open modal!!!!!!!!!')}
                        >
                            {'Link Channel'}
                        </button>
                    </div>
                </div>
            )}
        </div>
    );
};

export default Rhs;
