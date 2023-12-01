import React, {useEffect} from 'react';

import Constants from 'constants/index';
import usePluginApi from 'hooks/usePluginApi';

//global styles
import 'styles/main.scss';

/**
 * This is main App component for plugin
 * @returns {JSX.Element}
 */
const App = (): JSX.Element => {
    const {makeApiRequestWithCompletionStatus} = usePluginApi();

    useEffect(() => {
        makeApiRequestWithCompletionStatus(Constants.pluginApiServiceConfigs.whitelistUser.apiServiceName);
    }, []);

    return <></>;
};

export default App;
