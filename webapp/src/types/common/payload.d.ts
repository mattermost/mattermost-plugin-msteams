interface PaginationQueryParams {
    page: number;
    per_page: number;
}

type UnlinkChannelParams = {
    channelId: string;
}

interface SearchParams extends PaginationQueryParams {
    search?: string;
}

interface SearchMSChannelsParams extends SearchParams {
    teamId: string;
}

type LinkChannelsPayload = {
    mattermostTeamID: string,
    mattermostChannelID: string,
    msTeamsTeamID: string,
    msTeamsChannelID: string,
}
