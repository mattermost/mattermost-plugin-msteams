CREATE TABLE IF NOT EXISTS msteamssync_links (
    mmChannelID VARCHAR(255) PRIMARY KEY,
    mmTeamID VARCHAR(255),
    msTeamsChannelID VARCHAR(255),
    msTeamsTeamID VARCHAR(255),
    creator VARCHAR(255)
);

ALTER TABLE msteamssync_links ADD COLUMN IF NOT EXISTS creator VARCHAR(255);
