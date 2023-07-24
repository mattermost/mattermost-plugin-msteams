import {combineReducers} from 'redux';

import {msTeamsPluginApi} from '../services';

import apiRequestCompletionSlice from './apiRequest';
import connectedReducer from './connectedState';

export default combineReducers({
    apiRequestCompletionSlice,
    connectedReducer,
    [msTeamsPluginApi.reducerPath]: msTeamsPluginApi.reducer,
});
