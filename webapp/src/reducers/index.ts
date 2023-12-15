import {combineReducers} from 'redux';

import {msTeamsPluginApi} from 'services';

import apiRequestCompletionSlice from 'reducers/apiRequest';
import connectedStateSlice from 'reducers/connectedState';
import snackbarSlice from 'reducers/snackbar';
import rhsLoadingSlice from 'reducers/spinner';
import linkModalSlice from 'reducers/linkModal';
import refetchSlice from 'reducers/refetchState';

export default combineReducers({
    apiRequestCompletionSlice,
    connectedStateSlice,
    snackbarSlice,
    rhsLoadingSlice,
    linkModalSlice,
    refetchSlice,
    [msTeamsPluginApi.reducerPath]: msTeamsPluginApi.reducer,
});
