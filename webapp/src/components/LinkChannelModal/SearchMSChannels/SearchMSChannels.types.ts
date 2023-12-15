export type SearchMSChannelProps = {
    setChannel: React.Dispatch<React.SetStateAction<MSTeamOrChannel | null>>;
    teamId?: string | null,
}
