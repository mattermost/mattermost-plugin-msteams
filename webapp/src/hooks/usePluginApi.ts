import {useSelector, useDispatch} from 'react-redux';

import {useCallback} from 'react';

import {ReduxState} from 'types/common/store.d';

import {setApiRequestCompletionState} from 'reducers/apiRequest';
import {msTeamsPluginApi} from 'services';

function usePluginApi() {
    const state = useSelector((reduxState: ReduxState) => reduxState['plugins-com.mattermost.msteams-sync']);
    const dispatch = useDispatch();

    // Pass payload in POST requests only. For GET requests, there is no need to pass a payload argument
    const makeApiRequest = useCallback(async (serviceName: PluginApiServiceName, payload: APIRequestPayload): Promise<any> => {
        return dispatch(msTeamsPluginApi.endpoints[serviceName].initiate(payload));
    }, [dispatch, msTeamsPluginApi.endpoints]);

    const makeApiRequestWithCompletionStatus = useCallback(async (serviceName: PluginApiServiceName, payload: APIRequestPayload) => {
        const apiRequest = await makeApiRequest(serviceName, payload);

        if (apiRequest) {
            dispatch(setApiRequestCompletionState(serviceName));
        }
    }, [dispatch, makeApiRequest, setApiRequestCompletionState]);

    // Pass payload in POST requests only. For GET requests, there is no need to pass a payload argument
    const getApiState = useCallback((serviceName: PluginApiServiceName, payload: APIRequestPayload) => {
        const {data, isError, isLoading, isSuccess, error, isUninitialized} = msTeamsPluginApi.endpoints[serviceName].select(payload)(state);
        return {data, isError, isLoading, isSuccess, error, isUninitialized};
    }, [state, msTeamsPluginApi.endpoints]);

    return {makeApiRequest, makeApiRequestWithCompletionStatus, getApiState, state};
}

export default usePluginApi;
