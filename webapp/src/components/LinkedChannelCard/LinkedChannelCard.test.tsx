import React from 'react';

import {render, RenderResult} from '@testing-library/react';

import {LinkedChannelCard} from './LinkedChannelCard.component';
import {LinkedChannelCardProps} from './LinkedChannelCard.types';

const linkedChannelCardProps: LinkedChannelCardProps = {
    channelId: 'mockChannelId',
    mattermostChannelName: 'mockMattermostChannelName',
    mattermostTeamName: 'mockMattermostTeamName',
    msTeamsChannelName: 'mockMSTeamsChannelName',
    msTeamsTeamName: 'mockMSTeamsTeamName',
    mattermostChannelType: 'mattermostChannelType',
};

let tree: RenderResult;

describe('Linked Channel Card component', () => {
    beforeEach(() => {
        tree = render(<LinkedChannelCard {...linkedChannelCardProps}/>);
    });

    it('Should render correctly', () => {
        expect(tree).toMatchSnapshot();
    });

    it('Should show correct linked channel details', () => {
        expect(tree.getByText(linkedChannelCardProps.mattermostChannelName)).toBeVisible();
        expect(tree.getByText(linkedChannelCardProps.mattermostTeamName)).toBeVisible();
        expect(tree.getByText(linkedChannelCardProps.msTeamsChannelName)).toBeVisible();
        expect(tree.getByText(linkedChannelCardProps.msTeamsTeamName)).toBeVisible();
    });
});
