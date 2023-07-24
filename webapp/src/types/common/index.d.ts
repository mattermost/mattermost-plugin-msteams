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
    msTeamsTeamName: string,
    msTeamsChannelName: string,
    mattermostTeamName: string,
    mattermostChannelName: string,
    channelType: string,
}
