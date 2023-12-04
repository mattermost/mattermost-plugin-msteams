import {combineReducers} from 'redux';

import {msTeamsPluginApi} from 'services';

import apiRequestCompletionSlice from 'reducers/apiRequest';
import connectedStateSlice from 'reducers/connectedState';

export default combineReducers({
    apiRequestCompletionSlice,
    connectedStateSlice,
    [msTeamsPluginApi.reducerPath]: msTeamsPluginApi.reducer,
});
