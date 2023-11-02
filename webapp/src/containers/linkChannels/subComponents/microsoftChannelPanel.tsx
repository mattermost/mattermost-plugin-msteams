import React from 'react';

import {AutoComplete, ListItemType} from '@brightscout/mattermost-ui-library';

import {ChannelPanelProps} from './mattermostChannelPanel';

const MicrosoftChannelPanel = ({
    className = '',
    channelOptions,
    setChannelOptions,
    channel,
    setChannel,
    placeholder,
}: ChannelPanelProps): JSX.Element => {
    const handleChannelSelect = (_: any, option: ListItemType) => {
        setChannel(option.value);
    };

    return (
        <div className={className}>
            <AutoComplete
                fullWidth={true}
                items={channelOptions}
                label={placeholder}
                onSelect={handleChannelSelect}
                value={channel as string}
            />
        </div>
    );
};

export default MicrosoftChannelPanel;
