import {combineReducers} from 'redux';

import {msTeamsPluginApi} from 'services';

import apiRequestCompletionSlice from 'reducers/apiRequest';
import connectedStateSlice from 'reducers/connectedState';

import snackbarSlice from 'reducers/snackbar';

import needsConnectStateSlice from './needsConnectState';

export default combineReducers({
    apiRequestCompletionSlice,
    connectedStateSlice,
    needsConnectStateSlice,
    snackbarSlice,
    [msTeamsPluginApi.reducerPath]: msTeamsPluginApi.reducer,
});
