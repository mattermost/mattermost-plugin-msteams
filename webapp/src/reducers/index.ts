import {combineReducers} from 'redux';

import {msTeamsPluginApi} from '../services';

import apiRequestCompletionSlice from './apiRequest';
import connectedReducer from './connectedState';
import refetchReducer from './refetchState';

export default combineReducers({
    apiRequestCompletionSlice,
    connectedReducer,
    refetchReducer,
    [msTeamsPluginApi.reducerPath]: msTeamsPluginApi.reducer,
});
