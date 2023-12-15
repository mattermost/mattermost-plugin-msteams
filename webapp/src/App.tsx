import React, {useEffect, useMemo} from 'react';
import {Action, Store} from 'redux';
import {useDispatch} from 'react-redux';

import {GlobalState} from 'mattermost-redux/types/store';

import {RhsTitle} from 'components';

import {pluginApiServiceConfigs} from 'constants/apiService.constant';
import {defaultPage, defaultPerPage, pluginTitle, rhsButtonId} from 'constants/common.constants';
import {iconUrl} from 'constants/illustrations.constants';

import {Rhs} from 'containers';

import useApiRequestCompletionState from 'hooks/useApiRequestCompletionState';
import usePluginApi from 'hooks/usePluginApi';

import {setConnected} from 'reducers/connectedState';
import {setIsRhsLoading} from 'reducers/spinner';

// global styles
import 'styles/main.scss';

/**
 * This is the main App component for the plugin
 * @returns {JSX.Element}
 */
const App = ({registry, store}:{registry: PluginRegistry, store: Store<GlobalState, Action<Record<string, unknown>>>}): JSX.Element => {
    const dispatch = useDispatch();
    const {makeApiRequestWithCompletionStatus, getApiState} = usePluginApi();

    useEffect(() => {
        const linkedChannelsParams: SearchLinkedChannelParams = {page: defaultPage, per_page: defaultPerPage};

        makeApiRequestWithCompletionStatus(pluginApiServiceConfigs.getConfig.apiServiceName);
        makeApiRequestWithCompletionStatus(pluginApiServiceConfigs.needsConnect.apiServiceName);
        makeApiRequestWithCompletionStatus(pluginApiServiceConfigs.getLinkedChannels.apiServiceName, linkedChannelsParams);
    }, []);

    const {data: needsConnectData, isLoading} = useMemo(() => getApiState(pluginApiServiceConfigs.needsConnect.apiServiceName), [getApiState]);

    useEffect(() => {
        dispatch(setIsRhsLoading(isLoading));
    }, [isLoading]);

    const {data: configData} = useMemo(() => getApiState(pluginApiServiceConfigs.getConfig.apiServiceName), [getApiState]);

    useApiRequestCompletionState({
        serviceName: pluginApiServiceConfigs.needsConnect.apiServiceName,
        handleSuccess: () => {
            const data = needsConnectData as NeedsConnectData;
            dispatch(setConnected({connected: data.connected, username: data.username, msteamsUserId: data.msteamsUserId, isAlreadyConnected: data.connected}));
        },
    });

    useApiRequestCompletionState({
        serviceName: pluginApiServiceConfigs.getConfig.apiServiceName,
        handleSuccess: () => {
            const {rhsEnabled} = configData as ConfigResponse;
            const rhsButtonData = localStorage.getItem(rhsButtonId);

            // Unregister registered components and remove data present in the local storage.
            if (rhsButtonData) {
                const data = JSON.parse(rhsButtonData);
                registry.unregisterComponent(data.headerId);
                registry.unregisterComponent(data.appBarId);
                localStorage.removeItem(rhsButtonId);
            }

            // Register the right hand sidebar component, channel header button and app bar if the rhs is enabled.
            if (rhsEnabled) {
                let appBarId;
                const {_, toggleRHSPlugin} = registry.registerRightHandSidebarComponent(Rhs, <RhsTitle/>);
                const headerId = registry.registerChannelHeaderButtonAction(
                    <img
                        width={24}
                        height={24}
                        src={iconUrl}
                        style={{filter: 'grayscale(1)'}}
                    />, () => store.dispatch(toggleRHSPlugin), null, pluginTitle);

                if (registry.registerAppBarComponent) {
                    appBarId = registry.registerAppBarComponent(iconUrl, () => store.dispatch(toggleRHSPlugin), pluginTitle);
                }

                /**
                 * Store data in local storage to avoid extra registration of the above components.
                 * This was needed as on the load of the app, the plugin re-registers the above components, and multiple rhs buttons were becoming visible.
                 * We avoid this by keeping registered component id in local storage, unregistering on the load of the plugin (if registered previously), and registering them again.
                */
                localStorage.setItem(rhsButtonId, JSON.stringify({
                    headerId,
                    appBarId,
                }));
            }
        },
    });

    return <></>;
};

export default App;
