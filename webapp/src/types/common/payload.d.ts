type PaginationQueryParams = {
    page: number;
    per_page: number;
}

type UnlinkChannelParams = {
    channelId: string;
}

type SearchParams = PaginationQueryParams & {
    search?: string;
}

type SearchMSChannelsParams = SearchParams & {
    teamId: string;
}

type LinkChannelsPayload = {
    mattermostTeamID: string,
    mattermostChannelID: string,
    msTeamsTeamID: string,
    msTeamsChannelID: string,
}
