import React, { useCallback, useEffect, useState } from 'react';

import {Client4} from 'mattermost-redux/client';

import {MMSearch, ListItemType} from '@brightscout/mattermost-ui-library';
import {Channel} from 'src/containers/linkChannels/subComponents';
import Utils from 'src/utils';
import Constants from 'src/constants';

export type ChannelPanelProps = {
    className?: string;
    setChannel: (value: Channel | null) => void;
    placeholder: string;
    teamId: string | null,
}

const MattermostChannelPanel = ({
    className = '',
    setChannel,
    placeholder,
    teamId,
}: ChannelPanelProps): JSX.Element => {
    const [searchTerm, setSearchTerm] = useState<string>('');
    const [searchSuggestions, setSearchSuggestions] = useState<DropdownOptionType[]>([]);
    const [suggestionsLoading, setSuggestionsLoading] = useState<boolean>(false);

    useEffect(() => {
        handleClearInput();
    }, [teamId])

    const searchChannels = ({searchFor}: {searchFor?: string}) => {
        if(searchFor && teamId) {
            setSuggestionsLoading(true);
            Client4.autocompleteChannelsForSearch(teamId, searchFor)
            .then((channels) => {
                const suggestions = [];
                for(const channel of channels) {
                    suggestions.push({
                        label: channel.display_name,
                        value: channel.id,
                    })
                }
                setSearchSuggestions(suggestions);
                setSuggestionsLoading(false);
            }).catch((err) => {
                // TODO: Handle error here
                setSuggestionsLoading(false);
            })
        }
    }

    const debouncedSearchChannels = useCallback(Utils.debounce(searchChannels, Constants.DebounceFunctionTimeLimit), [searchChannels]);

    const handleSearch = (val: string) => {
        if(!val) {
            setSearchSuggestions([]);
        }
        setSearchTerm(val);
        debouncedSearchChannels({searchFor: val})
    }

    const handleChannelSelect = (_: any, option: ListItemType) => {
        setChannel({
            id: option.value,
            displayName: option.label as string,
        });
        setSearchTerm(option.label as string);
    };

    const handleClearInput = () => {
        setSearchTerm('');
        setSearchSuggestions([]);
        setChannel(null);
    }

    return (
        <div className={className}>
            <MMSearch
                label={placeholder}
                autoFocus={true}
                fullWidth={true}
                className={className}
                items={searchSuggestions}
                onSelect={handleChannelSelect}
                searchValue={searchTerm}
                setSearchValue={handleSearch}
                onClearInput={handleClearInput}
                optionsLoading={suggestionsLoading}
            />
        </div>
    );
};

export default MattermostChannelPanel;
