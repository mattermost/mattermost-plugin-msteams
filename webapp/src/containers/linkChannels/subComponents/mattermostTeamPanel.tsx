import React from 'react';

import {AutoComplete, ListItemType} from '@brightscout/mattermost-ui-library';

export type TeamPanelProps = {
    className?: string;
    teamOptions: DropdownOptionType[],
    setTeamOptions: (teamOptions: DropdownOptionType[]) => void;
    team: string | null;
    setTeam: (value: string | null) => void;
    placeholder?: string;
}

const MattermostTeamPanel = ({
    className = '',
    teamOptions,
    setTeamOptions,
    team,
    setTeam,
    placeholder,
}: TeamPanelProps): JSX.Element => {
    const handleTeamSelect = (_: any, option: ListItemType) => {
        setTeam(option.value);
    };

    return (
        <div className={className}>
            <AutoComplete
                fullWidth={true}
                items={teamOptions}
                label={placeholder}
                onSelect={handleTeamSelect}
                value={team as string}
            />
        </div>
    );
};

export default MattermostTeamPanel;
