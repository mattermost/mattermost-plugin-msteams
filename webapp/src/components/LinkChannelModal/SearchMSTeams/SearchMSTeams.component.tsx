import React, {useCallback, useEffect, useState} from 'react';

import {ListItemType, MMSearch} from '@brightscout/mattermost-ui-library';

import {useDispatch} from 'react-redux';

import {Icon} from 'components/Icon';
import {pluginApiServiceConfigs} from 'constants/apiService.constant';
import {debounceFunctionTimeLimitInMilliseconds, defaultPage, defaultPerPage} from 'constants/common.constants';
import useApiRequestCompletionState from 'hooks/useApiRequestCompletionState';
import usePluginApi from 'hooks/usePluginApi';
import utils from 'utils';
import {setLinkModalLoading} from 'reducers/linkModal';
import {getLinkModalState} from 'selectors';

export const SearchMSTeams = ({setMSTeam}: {setMSTeam: React.Dispatch<React.SetStateAction<MSTeamOrChannel | null>>}) => {
    const dispatch = useDispatch();
    const {makeApiRequestWithCompletionStatus, getApiState, state} = usePluginApi();
    const [searchTerm, setSearchTerm] = useState<string>('');
    const {msTeam} = getLinkModalState(state);
    const [searchTeamsPayload, setSearchTeamsPayload] = useState<SearchParams | null>(null);
    const [searchSuggestions, setSearchSuggestions] = useState<ListItemType[]>([]);

    useEffect(() => {
        setSearchTerm(msTeam);
    }, []);

    const searchTeams = ({searchFor}: {searchFor?: string}) => {
        if (searchFor) {
            const payload = {
                search: searchFor,
                page: defaultPage,
                per_page: defaultPerPage,
            };
            setSearchTeamsPayload(payload);
            makeApiRequestWithCompletionStatus(pluginApiServiceConfigs.searchMSTeams.apiServiceName, payload);
            dispatch(setLinkModalLoading(true));
        }
    };

    const debouncedSearchTeams = useCallback(utils.debounce(searchTeams, debounceFunctionTimeLimitInMilliseconds), [searchTeams]);

    const handleSearch = (val: string) => {
        if (!val) {
            setSearchSuggestions([]);
            setMSTeam(null);
        }
        setSearchTerm(val);
        debouncedSearchTeams({searchFor: val});
    };

    const handleTeamSelect = (_: any, option: ListItemType) => {
        setMSTeam({
            ID: option.value,
            DisplayName: option.label as string,
        });
        setSearchTerm(option.label as string);
    };

    const handleClearInput = () => {
        setSearchTerm('');
        setMSTeam(null);
        setSearchSuggestions([]);
    };

    const {data: searchedTeams, isLoading: searchSuggestionsLoading} = getApiState(pluginApiServiceConfigs.searchMSTeams.apiServiceName, searchTeamsPayload as SearchParams);
    useApiRequestCompletionState({
        serviceName: pluginApiServiceConfigs.searchMSTeams.apiServiceName,
        payload: searchTeamsPayload as SearchParams,
        handleSuccess: () => {
            if (searchedTeams) {
                const suggestions: ListItemType[] = [];
                for (const team of searchedTeams as MSTeamsSearchResponse) {
                    suggestions.push({
                        label: team.DisplayName,
                        value: team.ID,
                        icon: <Icon iconName='msTeams'/>,
                    });
                }
                setSearchSuggestions(suggestions);
            }
            dispatch(setLinkModalLoading(false));
        },
        handleError: () => {
            dispatch(setLinkModalLoading(false));
        },
    });

    return (
        <div className='d-flex flex-column gap-24'>
            <div className='d-flex gap-8 align-items-center'>
                <Icon iconName='msTeams'/>
                <h5 className='my-0 lh-20 wt-600'>{'Select a Microsoft Teams channel'}</h5>
            </div>
            <MMSearch
                fullWidth={true}
                label='Select a team in Microsoft Teams'
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
