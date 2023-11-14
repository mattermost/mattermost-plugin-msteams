import {combineReducers} from 'redux';

import {msTeamsPluginApi} from 'services';

import apiRequestCompletionSlice from 'reducers/apiRequest';
import connectedReducer from 'reducers/connectedState';

export default combineReducers({
    apiRequestCompletionSlice,
    connectedReducer,
    [msTeamsPluginApi.reducerPath]: msTeamsPluginApi.reducer,
});
