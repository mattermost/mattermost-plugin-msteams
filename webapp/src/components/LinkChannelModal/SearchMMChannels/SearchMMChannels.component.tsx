import React, {useCallback, useEffect, useState} from 'react';

import {Client4} from 'mattermost-redux/client';
import {channels as MMChannelTypes, teams as TeamsTypes} from 'mattermost-redux/types';
import {General as MMConstants} from 'mattermost-redux/constants';

import {ListItemType, MMSearch} from '@brightscout/mattermost-ui-library';

import {useDispatch, useSelector} from 'react-redux';

import utils from 'utils';

import {Icon} from 'components/Icon';

import {debounceFunctionTimeLimitInMilliseconds} from 'constants/common.constants';

import {setLinkModalLoading} from 'reducers/linkModal';

import {ReduxState} from 'types/common/store.d';

import usePluginApi from 'hooks/usePluginApi';
import {getLinkModalState} from 'selectors';

import {SearchMMChannelProps} from './SearchMMChannels.types';

export const SearchMMChannels = ({
    setChannel,
    teamId,
}: SearchMMChannelProps) => {
    const dispatch = useDispatch();
    const {state} = usePluginApi();
    const [searchTerm, setSearchTerm] = useState<string>('');
    const {teams} = useSelector((reduxState: ReduxState) => reduxState.entities);
    const {mmChannel} = getLinkModalState(state);

    const [searchSuggestions, setSearchSuggestions] = useState<ListItemType[]>([]);
    const [suggestionsLoading, setSuggestionsLoading] = useState<boolean>(false);

    useEffect(() => {
        if (!mmChannel) {
            handleClearInput();
        }
        setSearchTerm(mmChannel);
    }, [teamId]);

    const searchChannels = ({searchFor}: {searchFor?: string}) => {
        if (searchFor) {
            setSuggestionsLoading(true);
            dispatch(setLinkModalLoading(true));
            Client4.searchAllChannels(searchFor).
                then((channels) => {
                    const suggestions:ListItemType[] = [];
                    for (const channel of channels as MMChannelTypes.ChannelWithTeamData[]) {
                        suggestions.push({
                            label: channel.display_name,
                            value: channel.id,
                            secondaryLabel: channel.team_display_name,
                            icon: channel.type === MMConstants.PRIVATE_CHANNEL ? 'Lock' : 'Globe',
                        });
                    }
                    setSearchSuggestions(suggestions);
                }).finally(() => {
                    setSuggestionsLoading(false);
                    dispatch(setLinkModalLoading(false));
                });
        }
    };

    const debouncedSearchChannels = useCallback(utils.debounce(searchChannels, debounceFunctionTimeLimitInMilliseconds), [searchChannels]);

    const handleSearch = (val: string) => {
        if (!val) {
            setSearchSuggestions([]);
            setChannel(null);
        }
        setSearchTerm(val);
        debouncedSearchChannels({searchFor: val});
    };

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
    };

    return (
        <div className='d-flex flex-column gap-24'>
            <div className='d-flex gap-8 align-items-center'>
                <Icon iconName='mattermost'/>
                <h5 className='my-0 lh-20 wt-600'>{'Select a Mattermost channel'}</h5>
            </div>
            <MMSearch
                autoFocus={true}
                fullWidth={true}
                label='Search Mattermost channels'
                items={searchSuggestions}
                secondaryLabelPosition='inline'
                onSelect={handleChannelSelect}
                searchValue={searchTerm}
                setSearchValue={handleSearch}
                onClearInput={handleClearInput}
                optionsLoading={suggestionsLoading}
            />
        </div>
    );
};
