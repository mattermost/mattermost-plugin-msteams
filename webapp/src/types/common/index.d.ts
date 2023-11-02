type NeedsConnectData = {
    canSkip: boolean;
    needsConnect: boolean;
    connected: boolean;
    username: string;
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
    msTeamsChannelType: string,
}

type DropdownOptionType = {
    label?: string;
    value: string;
}
