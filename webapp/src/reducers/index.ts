import {combineReducers} from 'redux';

import {msTeamsPluginApi} from 'services';

import apiRequestCompletionSlice from 'reducers/apiRequest';

export default combineReducers({
    apiRequestCompletionSlice,
    [msTeamsPluginApi.reducerPath]: msTeamsPluginApi.reducer,
});
