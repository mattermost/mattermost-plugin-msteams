import {combineReducers} from 'redux';

import {msTeamsPluginApi} from 'src/services';

import apiRequestCompletionSlice from 'src/reducers/apiRequest';
import connectedReducer from 'src/reducers/connectedState';
import refetchReducer from 'src/reducers/refetchState';
import globalModalSlice from 'src/reducers/globalModal';

export default combineReducers({
    apiRequestCompletionSlice,
    connectedReducer,
    refetchReducer,
    globalModalSlice,
    [msTeamsPluginApi.reducerPath]: msTeamsPluginApi.reducer,
});
