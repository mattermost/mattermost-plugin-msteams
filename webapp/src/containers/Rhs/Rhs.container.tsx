import React, {useCallback, useEffect, useMemo, useState} from 'react';
import InfiniteScroll from 'react-infinite-scroll-component';
import {useDispatch, useSelector} from 'react-redux';

import {Button, Input, Spinner} from '@brightscout/mattermost-ui-library';

import {ReduxState} from 'types/common/store.d';

import {Dialog, Icon, IconName, LinkedChannelCard, Snackbar, WarningCard} from 'components';
import {pluginApiServiceConfigs} from 'constants/apiService.constant';
import {debounceSearchFunctionTimeLimitInMilliseconds, defaultPage, defaultPerPage} from 'constants/common.constants';
import Constants from 'constants/connectAccount.constants';
import {channelListTitle, noMoreChannelsText, noResultsFoundText} from 'constants/linkedChannels.constants';
import useApiRequestCompletionState from 'hooks/useApiRequestCompletionState';
import useAlert from 'hooks/useAlert';
import usePluginApi from 'hooks/usePluginApi';
import usePreviousState from 'hooks/usePreviousState';
import {getConnectedState, getIsRhsLoading, getSnackbarState} from 'selectors';
import {setConnected} from 'reducers/connectedState';
import utils from 'utils';

import './Rhs.styles.scss';

export const Rhs = () => {
    const {makeApiRequestWithCompletionStatus, getApiState, state} = usePluginApi();
    const globalState = useSelector((reduxState: ReduxState) => reduxState);

    // state variables
    const [totalLinkedChannels, setTotalLinkedChannels] = useState<ChannelLinkData[]>([]);
    const [paginationQueryParams, setPaginationQueryParams] = useState<PaginationQueryParams>({
        page: defaultPage,
        per_page: defaultPerPage,
    });
    const [getLinkedChannelsParams, setGetLinkedChannelsParams] = useState<SearchLinkedChannelParams | null>({...paginationQueryParams});
    const {connected, msteamsUserId, username, isAlreadyConnected} = getConnectedState(state);
    const [searchLinkedChannelsText, setSearchLinkedChannelsText] = useState('');
    const [firstRender, setFirstRender] = useState(true);
    const [isLinkedChannelsLoading, setIsLinkedChannelsLoading] = useState(false);

    const previousState = usePreviousState({searchLinkedChannelsText});

    const dispatch = useDispatch();
    const showAlert = useAlert();

    // Increase the page number by 1
    const handlePagination = () => setPaginationQueryParams({...paginationQueryParams, page: paginationQueryParams.page + 1});

    // Make api call to connect user account
    const connectAccount = useCallback(() => {
        makeApiRequestWithCompletionStatus(pluginApiServiceConfigs.connect.apiServiceName);
    }, []);

    // Make api call to disconnect user account
    const disconnectUser = useCallback(() => {
        makeApiRequestWithCompletionStatus(pluginApiServiceConfigs.disconnectUser.apiServiceName);
    }, []);

    // Reset the pagination params and empty the subscription list
    const resetStates = useCallback(() => {
        setPaginationQueryParams({page: defaultPage, per_page: defaultPerPage});
        setTotalLinkedChannels([]);
    }, []);

    // Check if more linked channels are present after the current page
    const hasMoreLinkedChannels = useMemo<boolean>(() => (
        (totalLinkedChannels.length - (paginationQueryParams.page * defaultPerPage) === defaultPerPage)
    ), [totalLinkedChannels]);

    // Show disconnect dialog component
    const [showDisconnectDialog, setShowDisconnectDialog] = useState(false);
    const {isRhsLoading} = getIsRhsLoading(state);
    const {isOpen} = getSnackbarState(state);
    const {data: linkedChannels, isLoading} = getApiState(pluginApiServiceConfigs.getLinkedChannels.apiServiceName, getLinkedChannelsParams as SearchLinkedChannelParams);
    const {isLoading: isUserDisconnecting} = getApiState(pluginApiServiceConfigs.disconnectUser.apiServiceName);

    // Handle searching of linked channels with debounce
    useEffect(() => {
        if (firstRender) {
            return;
        }

        const timer = setTimeout(() => {
            resetStates();
        }, debounceSearchFunctionTimeLimitInMilliseconds);

        /* eslint-disable consistent-return */
        return () => {
            clearTimeout(timer);
        };
    }, [searchLinkedChannelsText]);

    // Make api call to get linked channels
    useEffect(() => {
        const linkedChannelsParams: SearchLinkedChannelParams = {page: paginationQueryParams.page, per_page: paginationQueryParams.per_page};
        if (searchLinkedChannelsText) {
            linkedChannelsParams.search = searchLinkedChannelsText;
        }

        setGetLinkedChannelsParams(linkedChannelsParams);
        makeApiRequestWithCompletionStatus(pluginApiServiceConfigs.getLinkedChannels.apiServiceName, linkedChannelsParams);
        setIsLinkedChannelsLoading(true);
    }, [paginationQueryParams]);

    // Update connected state and show alert on successful connection of the user
    useEffect(() => {
        if (connected && !isAlreadyConnected) {
            showAlert({message: 'Your account is connected successfully.', severity: 'default'});
            dispatch(setConnected({connected, msteamsUserId, username, isAlreadyConnected: true}));
        }
    }, [connected, isAlreadyConnected]);

    // Update total linked channels after completion of the api to get linked channels
    useApiRequestCompletionState({
        serviceName: pluginApiServiceConfigs.getLinkedChannels.apiServiceName,
        payload: getLinkedChannelsParams as SearchLinkedChannelParams,
        handleSuccess: () => {
            if (linkedChannels) {
                setTotalLinkedChannels([...totalLinkedChannels, ...(linkedChannels as ChannelLinkData[])]);
            }
            if (firstRender && !paginationQueryParams.page) {
                setFirstRender(false);
            }
            setIsLinkedChannelsLoading(false);
        },
        handleError: () => setIsLinkedChannelsLoading(false),
    });

    // Disconnect a user and show alerts on completion of the api to disconnect the user
    useApiRequestCompletionState({
        serviceName: pluginApiServiceConfigs.disconnectUser.apiServiceName,
        handleSuccess: () => {
            dispatch(setConnected({connected: false, username: '', msteamsUserId: '', isAlreadyConnected: false}));
            setShowDisconnectDialog(false);
            showAlert({
                message: 'Your account has been disconnected.',
                severity: 'default',
            });
        },
        handleError: () => {
            showAlert({
                message: 'Error occurred while disconnecting the user.',
                severity: 'error',
            });
            setShowDisconnectDialog(false);
        },
    });

    // Get different states of rhs
    const getRhsView = useCallback(() => {
        // Show spinner in the rhs during loading
        if (isRhsLoading || (firstRender && isLoading)) {
            return (
                <div className='absolute d-flex align-items-center justify-center w-full h-full'>
                    <Spinner size='xl'/>
                </div>
            );
        }

        // Rhs state when user is disconnected and no linked channels are present
        if (!connected && !totalLinkedChannels.length && !searchLinkedChannelsText && !isLoading) {
            return (
                <div className='p-24 d-flex flex-column overflow-y-auto'>
                    <div className='flex-1 d-flex flex-column gap-16 align-items-center my-16'>
                        <div className='d-flex flex-column gap-16 align-items-center'>
                            <Icon
                                width={218}
                                iconName='connectAccount'
                            />
                            <h2 className='text-center wt-600 my-0'>{Constants.connectAccountMsg}</h2>
                        </div>
                        <Button onClick={connectAccount}>{Constants.connectButtonText}</Button>
                    </div>
                    <hr className='w-full my-32'/>
                    <div className='d-flex flex-column gap-24'>
                        <h5 className='my-0 wt-600'>{Constants.listTitle}</h5>
                        <ul className='my-0 px-0 d-flex flex-column gap-20'>
                            {Constants.connectAccountFeatures.map(({icon, text}) => (
                                <li
                                    className='d-flex gap-16 align-items-start'
                                    key={icon}
                                >
                                    <Icon iconName={icon as IconName}/>
                                    <h5 className='my-0 lh-20'>{text}</h5>
                                </li>
                            ))}
                        </ul>
                    </div>
                </div>
            );
        }

        /**
         * Rhs state for the following views:
         * user is disconnected and linked channels are present
         * user is connected and no linked channels are present
         * user is connected and linked channels are present
        */
        return (
            <div className='msteams-sync-rhs d-flex flex-column flex-1'>
                {connected ? (
                    <div className='py-12 px-20 border-y-1 d-flex gap-8'>
                        {/* TODO: Refactor user Avatar */}
                        <div
                            style={{
                                height: '32px',
                                width: '32px',
                                borderRadius: '50%',
                                backgroundColor: 'rgba(var(--center-channel-color-rgb), 0.12)',
                            }}
                        >
                            <img
                                style={{
                                    borderRadius: '50%',
                                }}
                                src={utils.getAvatarUrl(msteamsUserId, globalState.entities.general.config.SiteURL ?? '')}
                            />
                        </div>

                        <div>
                            <h5 className='my-0 font-12 lh-16'>{'Connected as '}<span className='wt-600'>{username}</span></h5>
                            <Button
                                size='sm'
                                variant='text'
                                className='p-0 lh-16'
                                onClick={() => setShowDisconnectDialog(true)}
                            >{'Disconnect'}</Button>
                        </div>
                    </div>
                ) : (
                    <div className='p-20 d-flex flex-column gap-20'>
                        <WarningCard
                            onConnect={connectAccount}
                        />
                    </div>
                )}
                {/* Show spinner during the first load of the linked channels. */}
                {isLinkedChannelsLoading && firstRender && (
                    <Spinner
                        size='xl'
                        className='scroll-container__spinner mt-10'
                    />
                )}
                {/* State when user is connected, but no linked channels are present. */}
                {!totalLinkedChannels.length && !isLinkedChannelsLoading && !searchLinkedChannelsText && !previousState?.searchLinkedChannelsText && (
                    <div className='d-flex align-items-center justify-center flex-1 flex-column px-40'>
                        {<>
                            <Icon iconName='noChannels'/>
                            <h3 className='my-0 lh-28 wt-600 text-center'>{'There are no linked channels yet'}</h3>
                        </>}
                    </div>
                )}
                {/* State when user is connected and linked channels are present. */}
                {((Boolean(totalLinkedChannels.length) || isLinkedChannelsLoading || searchLinkedChannelsText || previousState?.searchLinkedChannelsText) && !firstRender) && (
                    <>
                        <div className='lh-24 p-20 d-flex flex-column gap-16'>
                            <h4 className='font-16 my-0 wt-600'>{channelListTitle}</h4>
                            <div>
                                <Input
                                    iconName='MagnifyingGlass'
                                    label='Search for a channel'
                                    fullWidth={true}
                                    value={searchLinkedChannelsText}
                                    onChange={(e: React.ChangeEvent<HTMLInputElement>) => setSearchLinkedChannelsText(e.target.value)}
                                    onClose={() => setSearchLinkedChannelsText('')}
                                />
                            </div>
                        </div>
                        {/* Show a spinner while searching for a specific linked channel. */}
                        {isLinkedChannelsLoading && !paginationQueryParams.page ? (
                            <Spinner
                                size='xl'
                                className='scroll-container__spinner'
                            />
                        ) : (
                            <div
                                id='scrollableArea'
                                className='scroll-container flex-1-0-0'
                            >
                                <InfiniteScroll
                                    dataLength={totalLinkedChannels.length}
                                    next={handlePagination}
                                    hasMore={hasMoreLinkedChannels}
                                    loader={<Spinner className='scroll-container__spinner'/>}
                                    endMessage={
                                        <p className='text-center'>
                                            <b>{(searchLinkedChannelsText || previousState?.searchLinkedChannelsText) && !totalLinkedChannels.length ? noResultsFoundText : noMoreChannelsText}</b>
                                        </p>
                                    }
                                    scrollableTarget='scrollableArea'
                                >
                                    {totalLinkedChannels.map(({msTeamsChannelID, ...rest}) => (
                                        <LinkedChannelCard
                                            channelId={msTeamsChannelID}
                                            key={msTeamsChannelID}
                                            {...rest}
                                        />
                                    ))
                                    }
                                </InfiniteScroll>
                            </div>
                        )}
                    </>
                )}
                <Dialog
                    show={showDisconnectDialog}
                    destructive={true}
                    primaryButtonText='Disconnect'
                    secondaryButtonText='Cancel'
                    title='Disconnect Microsoft Teams Account'
                    isLoading={isUserDisconnecting}
                    onSubmitHandler={disconnectUser}
                    onCloseHandler={() => setShowDisconnectDialog(false)}
                >
                    {'Are you sure you want to disconnect your Microsoft Teams Account? You will no longer be able to send and receive messages to Microsoft Teams users from Mattermost.'}
                </Dialog>
            </div>
        );
    }, [connected, isRhsLoading, isLinkedChannelsLoading, totalLinkedChannels, firstRender, searchLinkedChannelsText, showDisconnectDialog]);

    return (
        <>
            <div
                className='msteams-sync-utils'
                style={{display: 'flex', flex: 1, flexDirection: 'column'}}
            >
                {getRhsView()}
            </div>
            {isOpen && <Snackbar/>}
        </>
    );
};
