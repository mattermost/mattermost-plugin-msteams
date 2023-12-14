import React, {useCallback, useMemo} from 'react';

import {Spinner} from '@brightscout/mattermost-ui-library';

import {pluginApiServiceConfigs} from 'constants/apiService.constant';

import usePluginApi from 'hooks/usePluginApi';

import {getConnectedState, getIsRhsLoading, getSnackbarState} from 'selectors';

import {Snackbar} from 'components';

import {ConnectAccount} from './views/ConnectAccount';
import {LinkedChannels} from './views/LinkedChannels';
import {ConnectedAccount} from './views/ConnectedAccount';

// TODO: update component later
export const Rhs = () => {
    const {state, getApiState} = usePluginApi();
    const {connected} = getConnectedState(state);
    const {isRhsLoading} = getIsRhsLoading(state);

    const {isOpen} = getSnackbarState(state);

    const {data: linkedChannels} = getApiState(pluginApiServiceConfigs.getLinkedChannels.apiServiceName);

    // NOTE: Commented out on purpose.This is part of Phase-II
    // const isAnyChannelLinked = useMemo(() => Boolean((linkedChannels as ChannelLinkData[])?.length), [linkedChannels]);
    const isAnyChannelLinked = false;

    const getRhsView = useCallback(() => {
        if (isRhsLoading) {
            return (
                <div className='msteams-sync-utils'>
                    <div className='absolute d-flex align-items-center justify-center w-full h-full'>
                        <Spinner size='xl'/>
                    </div>
                </div>
            );
        }

        if (!connected && !isAnyChannelLinked) {
            return <ConnectAccount/>;
        }

        if (!connected && isAnyChannelLinked) {
            return <LinkedChannels/>;
        }

        if (connected && !isAnyChannelLinked) {
            return <ConnectedAccount/>;
        }

        return <></>;
    }, [linkedChannels, connected, isRhsLoading]);

    return (
        <>
            {getRhsView()}
            {isOpen && <Snackbar/>}
        </>
    );
};
