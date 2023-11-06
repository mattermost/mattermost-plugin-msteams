import React from 'react';

import Constants from 'src/constants';
import {SVGIcons} from 'src/constants/icons';

type SummaryPanelProps = {
    className: string;
    mmTeam: string;
    mmChannel: string;
    msTeam: string;
    msChannel: string;
}

const SummaryPanel = ({
    className = '',
    mmTeam,
    mmChannel,
    msTeam,
    msChannel,
}: SummaryPanelProps): JSX.Element => {
    return (
        <div
            className={`summary-panel ${className}`}
        >
            <div className='summary-panel__mattermost_box box'>
                <div>
                    <img
                        className='box-icon'
                        src={Constants.mattermostHollowIconUrl}
                    />
                </div>
                <div className='team_container'>
                    <span>{mmTeam}</span>
                </div>
                <div className='channel_container'>
                    <span className='icon'>
                        {SVGIcons.globeIcon}
                    </span>
                    <span className='text'>{mmChannel}</span>
                </div>
            </div>
            <div className='link-icon'>
                {SVGIcons.linkIcon}
            </div>
            <div className='summary-panel__msteams_box box'>
                <div>
                    <img
                        className='box-icon'
                        src={Constants.msteamsIconUrl}
                    />
                </div>
                <div className='team_container'>
                    <span>{msTeam}</span>
                </div>
                <div className='channel_container'>
                    <img
                        className='icon'
                        src={Constants.iconUrl}
                    />
                    <span className='text'>{msChannel}</span>
                </div>
            </div>
        </div>
    );
};

export default SummaryPanel;
