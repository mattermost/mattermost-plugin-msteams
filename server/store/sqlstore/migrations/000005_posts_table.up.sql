CREATE TABLE IF NOT EXISTS msteamssync_posts (
    mmPostID VARCHAR(255) PRIMARY KEY,
    msTeamsPostID VARCHAR(255),
    msTeamsChannelID VARCHAR(255),
    msTeamsLastUpdateAt BIGINT
);
