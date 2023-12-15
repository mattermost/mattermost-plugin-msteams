export type LinkedChannelCardProps = Pick<ChannelLinkData, 'mattermostChannelName' | 'msTeamsChannelName' | 'msTeamsTeamName' | 'mattermostTeamName'> & {
    channelId: string
}
