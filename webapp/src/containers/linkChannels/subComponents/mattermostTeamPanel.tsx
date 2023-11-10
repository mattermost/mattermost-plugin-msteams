import React, {useEffect, useState} from 'react';

import {Client4} from 'mattermost-redux/client';

import {ListItemType, MMSearch} from '@brightscout/mattermost-ui-library';
import {Team} from 'src/containers/linkChannels/subComponents';

export type TeamPanelProps = {
    className?: string;
    setTeam: (value: Team | null) => void;
    placeholder: string;
}

const MattermostTeamPanel = ({
    className = '',
    setTeam,
    placeholder,
}: TeamPanelProps): JSX.Element => {
    const [searchTerm, setSearchTerm] = useState<string>('');
    const [searchSuggestions, setSearchSuggestions] = useState<DropdownOptionType[]>([]);
    const [isSearchDisabled, setIsSearchDisabled] = useState<boolean>(true);

    useEffect(() => {
        Client4.getMyTeams().then((teams) => {
            const suggestions = [];
            for(const team of teams) {
                suggestions.push({
                    label: team.display_name,
                    value: team.id,
                })
            }
            setSearchSuggestions(suggestions);
            setIsSearchDisabled(false);
        }).catch((err) => {
            // TODO: Handle error here
            setIsSearchDisabled(false);
        })
    }, [])

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
    }

    return (
        <div className={className}>
            <MMSearch
                disabled={isSearchDisabled}
                label={placeholder}
                autoFocus={true}
                fullWidth={true}
                className={className}
                items={searchSuggestions}
                onSelect={handleTeamSelect}
                searchValue={searchTerm}
                setSearchValue={setSearchTerm}
                onClearInput={handleClearInput}
            />
        </div>
    );
};

export default MattermostTeamPanel;
