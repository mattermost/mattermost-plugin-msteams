export type LinkedChannelCardProps = Pick<ChannelLinkData, 'mattermostChannelName' | 'msTeamsChannelName' | 'msTeamsTeamName' | 'mattermostTeamName' | 'mattermostChannelType'> & {
    channelId: string
}
