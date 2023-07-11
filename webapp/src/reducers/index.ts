import {combineReducers} from 'redux';

import {msTeamsPluginApi} from '../services';

import apiRequestCompletionSlice from './apiRequest';

export default combineReducers({
    apiRequestCompletionSlice,
    [msTeamsPluginApi.reducerPath]: msTeamsPluginApi.reducer,
});
