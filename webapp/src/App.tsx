import React, {useEffect} from 'react';
import {useDispatch} from 'react-redux';

import usePluginApi from 'hooks/usePluginApi';

//global styles
import {pluginApiServiceConfigs} from 'constants/apiService.constant';
import useApiRequestCompletionState from 'hooks/useApiRequestCompletionState';

import {setConnected} from 'reducers/connectedState';
import {defaultPage, defaultPerPage} from 'constants/common.constants';
import {setNeedsConnect} from 'reducers/needsConnectState';

import 'styles/main.scss';

/**
 * This is main App component for plugin
 * @returns {JSX.Element}
 */
const App = (): JSX.Element => {
    const dispatch = useDispatch();
    const {makeApiRequestWithCompletionStatus, getApiState} = usePluginApi();

    useEffect(() => {
        const linkedChannelsParams: SearchLinkedChannelParams = {page: defaultPage, per_page: defaultPerPage};

        makeApiRequestWithCompletionStatus(pluginApiServiceConfigs.whitelistUser.apiServiceName);
        makeApiRequestWithCompletionStatus(pluginApiServiceConfigs.needsConnect.apiServiceName);
        makeApiRequestWithCompletionStatus(pluginApiServiceConfigs.getLinkedChannels.apiServiceName, linkedChannelsParams);
    }, []);

    const {data: needsConnectData} = getApiState(pluginApiServiceConfigs.needsConnect.apiServiceName);

    useApiRequestCompletionState({
        serviceName: pluginApiServiceConfigs.needsConnect.apiServiceName,
        handleSuccess: () => {
            const data = needsConnectData as NeedsConnectData;
            dispatch(setConnected({connected: data.connected, username: data.username, msteamsUserId: data.msteamsUserId}));
            dispatch(setNeedsConnect({needsConnect: data.needsConnect}));
        },
    });

    return <></>;
};

export default App;
