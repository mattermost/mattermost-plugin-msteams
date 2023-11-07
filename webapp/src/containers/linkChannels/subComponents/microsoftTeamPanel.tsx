import React, { useState } from 'react';

import {MMSearch, ListItemType} from '@brightscout/mattermost-ui-library';

import {TeamPanelProps} from './mattermostTeamPanel';

const MicrosoftTeamPanel = ({
    className = '',
    teamOptions,
    optionsLoading = false,
    setTeam,
    placeholder,
}: TeamPanelProps): JSX.Element => {
    const [searchTerm, setSearchTerm] = useState<string>('');

    const handleTeamSelect = (_: any, option: ListItemType) => {
        setTeam(option.value);
        setSearchTerm(option.value);
    };

    const handleClearInput = () => {
        setSearchTerm('');
        setTeam(null);
    }

    return (
        <div className={className}>
            <MMSearch
                label={placeholder}
                autoFocus={true}
                fullWidth={true}
                className={className}
                items={teamOptions}
                onSelect={handleTeamSelect}
                searchValue={searchTerm}
                setSearchValue={setSearchTerm}
                optionsLoading={optionsLoading}
                onClearInput={handleClearInput}
            />
        </div>
    );
};

export default MicrosoftTeamPanel;
