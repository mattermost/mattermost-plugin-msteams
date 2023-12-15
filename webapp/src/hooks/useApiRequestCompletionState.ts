import {useEffect} from 'react';
import {useDispatch, useSelector} from 'react-redux';

import {ReduxState} from 'types/common/store.d';

import {resetApiRequestCompletionState} from 'reducers/apiRequest';
import {msTeamsPluginApi} from 'services';

type Props = {
    handleSuccess?: () => void
    handleError?: (error: APIError) => void
    serviceName: PluginApiServiceName
    payload?: APIRequestPayload,
}

function useApiRequestCompletionState({handleSuccess, handleError, serviceName, payload}: Props) {
    const state = useSelector((reduxState: ReduxState) => reduxState['plugins-com.mattermost.msteams-sync']);
    const apiState = msTeamsPluginApi.endpoints[serviceName].select(payload)(state);

    const requests = state.apiRequestCompletionSlice.requests;
    const dispatch = useDispatch();

    // Observe for the change in redux state after API call and do the required actions
    useEffect(() => {
        if (
            requests.includes(serviceName) &&
            apiState
        ) {
            const {isError, isSuccess, isUninitialized, error} = apiState;
            if (isSuccess && !isError) {
                handleSuccess?.();
            }

            if (!isSuccess && isError) {
                handleError?.(error as APIError);
            }

            if (!isUninitialized) {
                dispatch(resetApiRequestCompletionState(serviceName));
            }
        }
    }, [
        requests.includes(serviceName),
        apiState,
        resetApiRequestCompletionState(serviceName),
    ]);
}

export default useApiRequestCompletionState;
