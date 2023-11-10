import React, { useCallback, useEffect, useState } from 'react';

import {MMSearch, ListItemType} from '@brightscout/mattermost-ui-library';

import {ChannelPanelProps} from './mattermostChannelPanel';
import Utils from 'src/utils';
import Constants from 'src/constants';
import useApiRequestCompletionState from 'src/hooks/useApiRequestCompletionState';
import usePluginApi from 'src/hooks/usePluginApi';

const MicrosoftChannelPanel = ({
    className = '',
    setChannel,
    placeholder,
    teamId,
}: ChannelPanelProps): JSX.Element => {
    const {makeApiRequestWithCompletionStatus, getApiState} = usePluginApi();
    const [searchTerm, setSearchTerm] = useState<string>('');
    const [searchChannelsPayload, setSearchChannelsPayload] = useState<SearchMSChannelsParams | null>(null);
    const [searchSuggestions, setSearchSuggestions] = useState<DropdownOptionType[]>([]);

    useEffect(() => {
        handleClearInput();
    }, [teamId])

    const searchChannels = ({searchFor}: {searchFor?: string}) => {
        if(searchFor && teamId) {
            const payload = {
                search: searchFor,
                page: Constants.DefaultPage,
                per_page: Constants.DefaultPerPage,
                teamId,
            }
            setSearchChannelsPayload(payload);
            makeApiRequestWithCompletionStatus(Constants.pluginApiServiceConfigs.searchMSChannels.apiServiceName, payload);
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
        setChannel(null);
        setSearchSuggestions([]);
    }

    const {data: searchedChannels, isLoading: searchSuggestionsLoading} = getApiState(Constants.pluginApiServiceConfigs.searchMSChannels.apiServiceName, searchChannelsPayload as SearchMSChannelsParams);
    useApiRequestCompletionState({
        serviceName: Constants.pluginApiServiceConfigs.searchMSChannels.apiServiceName,
        payload: searchChannelsPayload as SearchMSChannelsParams,
        handleSuccess: () => {
            if(searchedChannels) {
                const suggestions: DropdownOptionType[] = [];
                for(const channel of searchedChannels as MSTeamsSearchResponse) {
                    suggestions.push({
                        label: channel.display_name,
                        value: channel.id,
                    })
                }
                setSearchSuggestions(suggestions);
            }
        },
        handleError: (error) => {
            // TODO: Handle this error
        }
    });

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
                optionsLoading={searchSuggestionsLoading}
            />
        </div>
    );
};

export default MicrosoftChannelPanel;
