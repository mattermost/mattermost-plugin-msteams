
type PaginationQueryParams = {
    page: number;
    per_page: number;
}

type UnlinkChannelParams = {
    channelId: string;
}

type SearchLinkedChannelParams = PaginationQueryParams & {
    search?: string;
}
