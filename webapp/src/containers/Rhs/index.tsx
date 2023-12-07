import React from 'react';

import Constants from 'constants/index';
import usePluginApi from 'hooks/usePluginApi';

// TODO: update component later
const Rhs = (): JSX.Element => {
    const {getApiState} = usePluginApi();

    const {data} = getApiState(Constants.pluginApiServiceConfigs.whitelistUser.apiServiceName);

    const {presentInWhitelist} = data as WhitelistUserResponse;

    return (<>
        {
            presentInWhitelist ?
                'MS Teams Sync plugin' : 'User Not present in Whitelist'
        }
    </>);
};

export default Rhs;
