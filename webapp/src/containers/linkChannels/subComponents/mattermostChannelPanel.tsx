import React, { useState } from 'react';

import {MMSearch, ListItemType} from '@brightscout/mattermost-ui-library';

export type ChannelPanelProps = {
    className?: string;
    channelOptions: DropdownOptionType[],
    setChannel: (value: string | null) => void;
    placeholder: string;
    optionsLoading?: boolean;
}

const MattermostChannelPanel = ({
    className = '',
    channelOptions,
    setChannel,
    placeholder,
    optionsLoading = false,
}: ChannelPanelProps): JSX.Element => {
    const [searchTerm, setSearchTerm] = useState<string>('');

    const handleChannelSelect = (_: any, option: ListItemType) => {
        setChannel(option.value);
        setSearchTerm(option.value);
    };

    const handleClearInput = () => {
        setSearchTerm('');
        setChannel(null);
    }

    return (
        <div className={className}>
            <MMSearch
                label={placeholder}
                autoFocus={true}
                fullWidth={true}
                className={className}
                items={channelOptions}
                onSelect={handleChannelSelect}
                searchValue={searchTerm}
                setSearchValue={setSearchTerm}
                optionsLoading={optionsLoading}
                onClearInput={handleClearInput}
            />
        </div>
    );
};

export default MattermostChannelPanel;
