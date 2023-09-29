import React, {useCallback, useEffect, useMemo, useState} from 'react';
import InfiniteScroll from 'react-infinite-scroll-component';
import {useDispatch} from 'react-redux';

import {Spinner, Tooltip, Button, Dialog, LinearProgress, Input} from '@brightscout/mattermost-ui-library';

import {General as MMConstants} from 'mattermost-redux/constants';

import {setConnected} from '../../reducers/connectedState';
import {refetch, resetRefetch} from '../../reducers/refetchState';
import useApiRequestCompletionState from '../../hooks/useApiRequestCompletionState';
import usePluginApi from '../../hooks/usePluginApi';

import Constants from '../../constants';
import {SVGIcons} from '../../constants/icons';

import './rhs.scss';

const Rhs = (): JSX.Element => {
    const {state, makeApiRequestWithCompletionStatus, getApiState} = usePluginApi();
    const refetchLinkedChannels = state.refetchReducer.refetch;
    const {connected, username} = state.connectedReducer;
    const dispatch = useDispatch();

    const [totalLinkedChannels, setTotalLinkedChannels] = useState<ChannelLinkData[]>([]);
    const [paginationQueryParams, setPaginationQueryParams] = useState<PaginationQueryParams>({
        page: Constants.DefaultPage,
        per_page: Constants.DefaultPerPage,
    });
    const [getLinkedChannelsParams, setGetLinkedChannelsParams] = useState<PaginationQueryParams | null>(null);
    const [showDialog, setShowDialog] = useState(false);
    const [dialogContent, setDialogContent] = useState('');
    const [showDestructivetDialog, setShowDestructiveDialog] = useState(false);
    const [primaryActionText, setPrimaryActionText] = useState('');
    const [unlinkChannelParams, setUnlinkChannelParams] = useState<UnlinkChannelParams | null>(null);
    const [searchLinkedChannelsText, setSearchLinkedChannelsText] = useState('');
    const [firstRender, setFirstRender] = useState(true);

    const connectAccount = useCallback(() => {
        makeApiRequestWithCompletionStatus(Constants.pluginApiServiceConfigs.connect.apiServiceName);
    }, []);

    const disconnectUser = useCallback(() => {
        makeApiRequestWithCompletionStatus(Constants.pluginApiServiceConfigs.disconnectUser.apiServiceName);
    }, []);

    // Reset the pagination params and empty the subscription list
    const resetStates = useCallback(() => {
        setPaginationQueryParams({page: Constants.DefaultPage, per_page: Constants.DefaultPerPage});
        setTotalLinkedChannels([]);
    }, []);

    const showDefaultDialog = useCallback(() => {
        setShowDestructiveDialog(false);
    }, []);

    const unlinkChannel = () => {
        makeApiRequestWithCompletionStatus(Constants.pluginApiServiceConfigs.unlinkChannel.apiServiceName, {channelId: unlinkChannelParams?.channelId ?? ''});
    };

    useEffect(() => {
        setFirstRender(false);
    }, []);

    useEffect(() => {
        if (firstRender) {
            return;
        }

        const timer = setTimeout(() => {
            resetStates();
        }, Constants.DebounceFunctionTimeLimit);
        return () => {
            clearTimeout(timer);
        };
    }, [searchLinkedChannelsText]);

    useEffect(() => {
        const linkedChannelsParams: SearchLinkedChannelParams = {search: searchLinkedChannelsText, page: paginationQueryParams.page, per_page: paginationQueryParams.per_page};
        setGetLinkedChannelsParams(linkedChannelsParams);
        makeApiRequestWithCompletionStatus(Constants.pluginApiServiceConfigs.getLinkedChannels.apiServiceName, linkedChannelsParams);
    }, [paginationQueryParams]);

    useEffect(() => {
        if (refetchLinkedChannels) {
            resetStates();
            dispatch(resetRefetch());
        }
    }, [refetchLinkedChannels]);

    const {data: connectData} = getApiState(Constants.pluginApiServiceConfigs.connect.apiServiceName);
    const {data: linkedChannels, isLoading: linkedChannelsLoading} = getApiState(Constants.pluginApiServiceConfigs.getLinkedChannels.apiServiceName, getLinkedChannelsParams as PaginationQueryParams);
    const {data: disconnectUserData, isLoading: isDisconnectUserLoading} = getApiState(Constants.pluginApiServiceConfigs.disconnectUser.apiServiceName);
    const {data: unlinkChannelData, isLoading: isUnlinkChannelLoading} = getApiState(Constants.pluginApiServiceConfigs.unlinkChannel.apiServiceName, unlinkChannelParams as UnlinkChannelParams);

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

    useApiRequestCompletionState({
        serviceName: Constants.pluginApiServiceConfigs.disconnectUser.apiServiceName,
        handleSuccess: () => {
            dispatch(setConnected({connected: false, username: ''}));
            setDialogContent(disconnectUserData as string);
            showDefaultDialog();
        },
        handleError: (disconnectUserError) => {
            showDefaultDialog();
            setDialogContent(disconnectUserError.data);
        },
    });

    useApiRequestCompletionState({
        serviceName: Constants.pluginApiServiceConfigs.unlinkChannel.apiServiceName,
        payload: unlinkChannelParams as UnlinkChannelParams,
        handleSuccess: () => {
            setDialogContent(unlinkChannelData as string);
            showDefaultDialog();
            dispatch(refetch());
        },
        handleError: (unlinkChannelError) => {
            showDefaultDialog();
            setDialogContent(unlinkChannelError.data);
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
                <div className='rhs-disconnect'>
                    <div className='rhs-disconnect__heading'>{'Connected account'}</div>
                    <div className='rhs-disconnect__body'>
                        <div className='rhs-disconnect__sub-body'>
                            <img src={Constants.msteamsIconUrl}/>
                            <div className='rhs-disconnect__title'>{username}</div>
                        </div>
                        <Button
                            className='rhs-disconnect__disconnect-button'
                            onClick={() => {
                                setDialogContent('Are you sure you want to disconnect your MS Teams Account?');
                                setShowDialog(true);
                                setShowDestructiveDialog(true);
                                setPrimaryActionText('Disconnect');
                            }}
                            variant='secondary'
                        >
                            {'Disconnect'}
                        </Button>
                    </div>
                </div>
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
                <div className={`rhs-body ${connected ? 'rhs-body__connect-body' : 'rhs-body__disconnect-body'}`}>
                    <div className='rhs-body__title'>{'Linked Channels'}</div>
                    <div className='rhs-body__subtitle'>{'Messages will be synchronized between linked channels.'}</div>
                    <Input
                        iconName='MagnifyingGlass'
                        label='Search'
                        fullWidth={true}
                        value={searchLinkedChannelsText}
                        onChange={(e: React.ChangeEvent<HTMLInputElement>) => setSearchLinkedChannelsText(e.target.value)}
                        onClose={() => setSearchLinkedChannelsText('')}
                    />
                    {linkedChannelsLoading && !paginationQueryParams.page && <Spinner className='rhs-body__spinner'/>}
                    {Boolean(totalLinkedChannels.length) && (
                        <div className='link-data__container'>
                            <div className='link-data__title'>
                                <img src={Constants.mattermostIconUrl}/>
                                <div className='link-data__title-values'>{'Mattermost'}</div>
                                <img src={Constants.msteamsIconUrl}/>
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
                                            <div className={`link-data__mm-values ${link.mattermostChannelType === MMConstants.PRIVATE_CHANNEL && 'link-data__private-data-width'}`}>
                                                <div className='link-data__mm-icons'><i className={`${link.mattermostChannelType === MMConstants.PRIVATE_CHANNEL ? 'icon icon-lock-outline' : 'icon icon-globe'}`}/></div>
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
                                                <div className='link-data__ms-icon'>{link.msTeamsChannelType === MMConstants.PRIVATE_CHANNEL && (
                                                    <img
                                                        className='msteams-private-channel-icon'
                                                        src={Constants.msteamsPrivateChannelIconUrl}
                                                    />
                                                )}
                                                </div>
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
                                                    onClick={() => {
                                                        setDialogContent('Are you sure you want to unlink this channel?');
                                                        setShowDialog(true);
                                                        setShowDestructiveDialog(true);
                                                        setPrimaryActionText('Unlink');
                                                        setUnlinkChannelParams({channelId: link.mattermostChannelID});
                                                    }}
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
                    {!totalLinkedChannels.length && !linkedChannelsLoading && (
                        <div className='no-link'>
                            {SVGIcons.globeIcon}
                            <div className='no-link__title'>{'There are no linked channels'}</div>
                            {!connected && (
                                <button
                                    className='btn btn-primary'

                                    // TODO: Update later
                                    // eslint-disable-next-line no-alert
                                    onClick={() => alert('open modal!!!!!!!!!')}
                                >
                                    {'Link Channel'}
                                </button>
                            )}
                        </div>
                    )}
                </div>
            </div>
            {connected && (
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
            <Dialog
                description={dialogContent}
                destructive={showDestructivetDialog}
                show={showDialog}
                primaryActionText={showDestructivetDialog && primaryActionText}
                onCloseHandler={() => {
                    setShowDialog(false);
                    setTimeout(() => {
                        setPrimaryActionText('');
                        setShowDestructiveDialog(false);
                        setDialogContent('');
                    }, 500);
                }}
                onSubmitHandler={primaryActionText === 'Disconnect' ? disconnectUser : unlinkChannel}
                className='disconnect-dialog'
            >
                {(isDisconnectUserLoading || isUnlinkChannelLoading) && <LinearProgress/>}
            </Dialog>
        </div>
    );
};

export default Rhs;
