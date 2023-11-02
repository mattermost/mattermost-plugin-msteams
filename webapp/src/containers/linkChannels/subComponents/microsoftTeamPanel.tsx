import React from 'react';

import {AutoComplete, ListItemType} from '@brightscout/mattermost-ui-library';

import {TeamPanelProps} from './mattermostTeamPanel';

const MicrosoftTeamPanel = ({
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

export default MicrosoftTeamPanel;
