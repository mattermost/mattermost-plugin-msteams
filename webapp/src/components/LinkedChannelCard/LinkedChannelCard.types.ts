export type LinkedChannelCardProps = Pick<ChannelLinkData, 'mattermostChannelName' | 'msTeamsChannelName' | 'msTeamsTeamName' | 'mattermostTeamName' | 'mattermostChannelType' | 'mattermostChannelID' | 'mattermostChannelType'> & {
    channelId: string
}
