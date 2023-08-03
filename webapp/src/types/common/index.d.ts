type NeedsConnectData = {
    canSkip: boolean;
    needsConnect: boolean;
    connected: boolean;
}

type ConnectData = {
    connectUrl: string;
}

type WebsocketEventParams = {
    event: string,
    data: Record<string, string>,
}

type ChannelLinkData = {
    msTeamsTeamID: string,
    msTeamsTeamName: string,
    msTeamsChannelID: string,
    msTeamsChannelName: string,
    mattermostTeamID: string,
    mattermostTeamName: string,
    mattermostChannelID: string,
    mattermostChannelName: string,
    mattermostChannelType: string,
}
