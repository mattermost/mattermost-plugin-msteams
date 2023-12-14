import React, {useCallback, useEffect} from 'react';
import {useDispatch} from 'react-redux';

import {Button} from '@brightscout/mattermost-ui-library';

import usePluginApi from 'hooks/usePluginApi';
import {pluginApiServiceConfigs} from 'constants/apiService.constant';
import useDialog from 'hooks/useDialog';
import useAlert from 'hooks/useAlert';
import useApiRequestCompletionState from 'hooks/useApiRequestCompletionState';
import {setConnected} from 'reducers/connectedState';
import {getConnectedState} from 'selectors';

import utils from 'utils';

export const ConnectedAccount = () => {
    const dispatch = useDispatch();
    const {makeApiRequestWithCompletionStatus, state} = usePluginApi();
    const {username, isAlreadyConnected, msteamsUserId, connected} = getConnectedState(state);

    const showAlert = useAlert();

    const disconnectUser = useCallback(() => {
        makeApiRequestWithCompletionStatus(pluginApiServiceConfigs.disconnectUser.apiServiceName);
    }, []);

    const {DialogComponent, showDialog, hideDialog} = useDialog({
        onSubmitHandler: disconnectUser,
        onCloseHandler: () => {
            hideDialog();
        },
    });

    useApiRequestCompletionState({
        serviceName: pluginApiServiceConfigs.disconnectUser.apiServiceName,
        handleSuccess: () => {
            dispatch(setConnected({connected: false, username: '', msteamsUserId: '', isAlreadyConnected: false}));
            hideDialog();
            showAlert({
                message: 'Your account has been disconnected.',
            });
        },
        handleError: () => {
            showAlert({
                message: 'Error occurred while disconnecting the user.',
                severity: 'error',
            });
            hideDialog();
        },
    });

    useEffect(() => {
        if (connected && !isAlreadyConnected) {
            showAlert({message: 'Your account is connected successfully.'});
            dispatch(setConnected({connected, msteamsUserId, username, isAlreadyConnected: true}));
        }
    }, [connected, isAlreadyConnected]);

    return (
        <div className='msteams-sync-utils'>
            <div className='flex-1 msteams-sync-rhs d-flex flex-column'>
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
                            src={utils.getAvatarUrl(msteamsUserId)}
                        />
                    </div>
                    <div>
                        <h5 className='my-0 font-12 lh-16'>{'Connected as '}<span className='wt-600'>{username}</span></h5>
                        <Button
                            size='sm'
                            variant='text'
                            className='p-0 lh-16'
                            onClick={() => showDialog({
                                destructive: true,
                                primaryButtonText: 'Disconnect',
                                secondaryButtonText: 'Cancel',
                                description: 'Are you sure you want to disconnect your Microsoft Teams Account? You will no longer be able to send and receive messages to Microsoft Teams users from Mattermost.',
                                isLoading: false,
                                title: 'Disconnect Microsoft Teams Account',
                            })}
                        >{'Disconnect'}</Button>
                    </div>
                </div>
                <div className='d-flex align-items-center justify-center flex-1 flex-column px-40'>
                    {/* NOTE: Part of Phase-II */}
                    {/* <Icon iconName='noChannels'/>
                <h3 className='my-0 lh-28 wt-600 text-center'>{'There are no linked channels yet'}</h3> */}
                </div>
                <DialogComponent/>
            </div>
        </div>
    );
};
