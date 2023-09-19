interface PaginationQueryParams {
    page: number;
    per_page: number;
}

type UnlinkChannelParams = {
    channelId: string;
}

interface SearchLinkedChannelParams extends PaginationQueryParams {
    search?: string;
}
