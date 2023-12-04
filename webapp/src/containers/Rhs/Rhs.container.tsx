import React, {useMemo} from 'react';

import {pluginApiServiceConfigs} from 'constants/apiService.constant';

import usePluginApi from 'hooks/usePluginApi';

import {getConnectedState} from 'selectors';

import {ConnectAccount} from './views/ConnectAccount';
import {LinkedChannels} from './views/LinkedChannels';

// TODO: update component later
export const Rhs = () => {
    const {state, getApiState} = usePluginApi();
    const {connected} = getConnectedState(state);

    const {data} = getApiState(pluginApiServiceConfigs.whitelistUser.apiServiceName);
    const {data: linkedChannels} = getApiState(pluginApiServiceConfigs.getLinkedChannels.apiServiceName);

    const {presentInWhitelist} = data as WhitelistUserResponse;
    const isAnyChannelLinked = useMemo(() => Boolean((linkedChannels as ChannelLinkData[])?.length), [linkedChannels]);

    const getRhsView = () => {
        if (!connected && !isAnyChannelLinked) {
            return <ConnectAccount/>;
        }

        if (!connected && isAnyChannelLinked) {
            return <LinkedChannels/>;
        }

        return <></>;
    };

    return (
        presentInWhitelist ?
            <>{'MS Teams Sync plugin'}</> : getRhsView()

    );
};
