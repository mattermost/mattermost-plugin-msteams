import {combineReducers} from 'redux';

import {msTeamsPluginApi} from 'services';

import apiRequestCompletionSlice from 'reducers/apiRequest';
import connectedStateSlice from 'reducers/connectedState';

import snackbarSlice from 'reducers/snackbar';

export default combineReducers({
    apiRequestCompletionSlice,
    connectedStateSlice,
    snackbarSlice,
    [msTeamsPluginApi.reducerPath]: msTeamsPluginApi.reducer,
});
