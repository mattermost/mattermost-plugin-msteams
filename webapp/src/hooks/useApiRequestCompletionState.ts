import {useEffect} from 'react';
import {useDispatch} from 'react-redux';

import {getApiRequestCompletionState} from 'selectors';
import {resetApiRequestCompletionState} from 'reducers/apiRequest';

import usePluginApi from 'hooks/usePluginApi';

type Props = {
    handleSuccess?: () => void
    handleError?: (error: APIError) => void
    serviceName: PluginApiServiceName
    payload?: APIRequestPayload
}

function useApiRequestCompletionState({handleSuccess, handleError, serviceName, payload}: Props) {
    const {getApiState, state} = usePluginApi();
    const dispatch = useDispatch();

    // Observe for the change in redux state after API call and do the required actions
    useEffect(() => {
        if (
            getApiRequestCompletionState(state).requests.includes(serviceName) &&
            getApiState(serviceName, payload)
        ) {
            const {isError, isSuccess, isUninitialized, error} = getApiState(serviceName, payload);
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
        getApiRequestCompletionState(state).requests.includes(serviceName),
        getApiState(serviceName, payload),
    ]);
}

export default useApiRequestCompletionState;
