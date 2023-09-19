import React from 'react';

import {useDispatch} from 'react-redux';

import usePluginApi from 'src/hooks/usePluginApi';
import {isLinkChannelsModalOpen} from 'src/selectors';
import {resetGlobalModalState} from 'src/reducers/globalModal';

import LinkChannelsModal from 'src/containers/linkChannels/subComponents';

const LinkChannels = () => {
    const dispatch = useDispatch();
    const {state: pluginState} = usePluginApi();

    console.log('14', pluginState);

    return (
        <LinkChannelsModal
            open={isLinkChannelsModalOpen(pluginState)}
            close={() => dispatch(resetGlobalModalState())}
        />
    );
};

export default LinkChannels;
