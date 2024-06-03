CREATE TABLE IF NOT EXISTS msteamssync_subscriptions (
    subscriptionID VARCHAR(255) PRIMARY KEY,
    type VARCHAR(255),
    msTeamsTeamID VARCHAR(255),
    msTeamsChannelID VARCHAR(255),
    msTeamsUserID VARCHAR(255),
    secret VARCHAR(255),
    expiresOn BIGINT
);
