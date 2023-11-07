import React, {useState} from 'react';

import {ListItemType, MMSearch} from '@brightscout/mattermost-ui-library';

export type TeamPanelProps = {
    className?: string;
    teamOptions: DropdownOptionType[],
    setTeam: (value: string | null) => void;
    placeholder: string;
    optionsLoading?: boolean;
}

const MattermostTeamPanel = ({
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

export default MattermostTeamPanel;
