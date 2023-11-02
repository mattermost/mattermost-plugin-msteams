import React from 'react';

import {AutoComplete, ListItemType} from '@brightscout/mattermost-ui-library';

export type ChannelPanelProps = {
    className?: string;
    channelOptions: DropdownOptionType[],
    setChannelOptions: (channelOptions: DropdownOptionType[]) => void;
    channel: string | null;
    setChannel: (value: string | null) => void;
    placeholder?: string;
}

const MattermostChannelPanel = ({
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
                disableResize={true}
                items={channelOptions}
                label={placeholder}
                onSelect={handleChannelSelect}
                value={channel as string}
            />
        </div>
    );
};

export default MattermostChannelPanel;
