type NeedsConnectData = {
    canSkip: boolean;
    needsConnect: boolean;
    connected: boolean;
    username: string;
    msteamsUserId: string;
}

type ConnectData = {
    connectUrl: string;
}

type WhitelistUserResponse= {
    presentInWhitelist: boolean
}

type ChannelLinkData = {
    msTeamsTeamID: string,
    msTeamsTeamName: string,
    msTeamsChannelID: string,
    msTeamsChannelName: string,
    msTeamsChannelType: string,
    mattermostTeamID: string,
    mattermostTeamName: string,
    mattermostChannelID: string,
    mattermostChannelName: string,
    mattermostChannelType: string,
}

type WebsocketEventParams = {
    event: string,
    data: Record<string, string>,
}
