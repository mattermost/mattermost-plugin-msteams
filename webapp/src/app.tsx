import React, {useEffect} from 'react';
import {useDispatch} from 'react-redux';

import usePluginApi from './hooks/usePluginApi';
import useApiRequestCompletionState from './hooks/useApiRequestCompletionState';

import Constants from './constants';

import {setConnected} from './reducers/connectedState';

const App = (): JSX.Element => {
    const {makeApiRequestWithCompletionStatus, getApiState} = usePluginApi();
    const dispatch = useDispatch();

    useEffect(() => {
        makeApiRequestWithCompletionStatus(Constants.pluginApiServiceConfigs.needsConnect.apiServiceName);
    }, []);

    const {data: needsConnectData} = getApiState(Constants.pluginApiServiceConfigs.needsConnect.apiServiceName);

    useApiRequestCompletionState({
        serviceName: Constants.pluginApiServiceConfigs.needsConnect.apiServiceName,
        handleSuccess: () => {
            const data = needsConnectData as NeedsConnectData;
            dispatch(setConnected({connected: data.connected, username: data.username}));
        },
    });

    // This container is used just for making the API call to check user connection, it doesn't render anything.
    return <></>;
};

export default App;
