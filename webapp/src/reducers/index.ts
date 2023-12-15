import {combineReducers} from 'redux';

import {msTeamsPluginApi} from 'services';

import apiRequestCompletionSlice from 'reducers/apiRequest';
import connectedStateSlice from 'reducers/connectedState';
import snackbarSlice from 'reducers/snackbar';
import rhsLoadingSlice from 'reducers/spinner';

export default combineReducers({
    apiRequestCompletionSlice,
    connectedStateSlice,
    snackbarSlice,
    rhsLoadingSlice,
    [msTeamsPluginApi.reducerPath]: msTeamsPluginApi.reducer,
});
