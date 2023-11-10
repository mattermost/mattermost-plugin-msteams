import React, { useCallback, useState } from 'react';

import {MMSearch, ListItemType} from '@brightscout/mattermost-ui-library';

import {TeamPanelProps} from './mattermostTeamPanel';
import Utils from 'src/utils';
import Constants from 'src/constants';
import useApiRequestCompletionState from 'src/hooks/useApiRequestCompletionState';
import usePluginApi from 'src/hooks/usePluginApi';

const MicrosoftTeamPanel = ({
    className = '',
    setTeam,
    placeholder,
}: TeamPanelProps): JSX.Element => {
    const {makeApiRequestWithCompletionStatus, getApiState} = usePluginApi();
    const [searchTerm, setSearchTerm] = useState<string>('');
    const [searchTeamsPayload, setSearchTeamsPayload] = useState<SearchParams | null>(null);
    const [searchSuggestions, setSearchSuggestions] = useState<DropdownOptionType[]>([]);

    const searchTeams = ({searchFor}: {searchFor?: string}) => {
        if(searchFor) {
            const payload = {
                search: searchFor,
                page: Constants.DefaultPage,
                per_page: Constants.DefaultPerPage,
            }
            setSearchTeamsPayload(payload);
            makeApiRequestWithCompletionStatus(Constants.pluginApiServiceConfigs.searchMSTeams.apiServiceName, payload);
        }
    }

    const debouncedSearchTeams = useCallback(Utils.debounce(searchTeams, Constants.DebounceFunctionTimeLimit), [searchTeams]);

    const handleSearch = (val: string) => {
        if(!val) {
            setSearchSuggestions([]);
        }
        setSearchTerm(val);
        debouncedSearchTeams({searchFor: val})
    }

    const handleTeamSelect = (_: any, option: ListItemType) => {
        setTeam({
            id: option.value,
            displayName: option.label as string,
        });
        setSearchTerm(option.label as string);
    };

    const handleClearInput = () => {
        setSearchTerm('');
        setTeam(null);
        setSearchSuggestions([]);
    }

    const {data: searchedTeams, isLoading: searchSuggestionsLoading} = getApiState(Constants.pluginApiServiceConfigs.searchMSTeams.apiServiceName, searchTeamsPayload as SearchParams);
    useApiRequestCompletionState({
        serviceName: Constants.pluginApiServiceConfigs.searchMSTeams.apiServiceName,
        payload: searchTeamsPayload as SearchParams,
        handleSuccess: () => {
            if(searchedTeams) {
                const suggestions: DropdownOptionType[] = [];
                for(const team of searchedTeams as MSTeamsSearchResponse) {
                    suggestions.push({
                        label: team.display_name,
                        value: team.id,
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
                onSelect={handleTeamSelect}
                searchValue={searchTerm}
                setSearchValue={handleSearch}
                onClearInput={handleClearInput}
                optionsLoading={searchSuggestionsLoading}
            />
        </div>
    );
};

export default MicrosoftTeamPanel;
