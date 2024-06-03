CREATE TABLE IF NOT EXISTS msteamssync_users (
    mmUserID VARCHAR(255),
    msTeamsUserID VARCHAR(255),
    token TEXT,

    PRIMARY KEY(mmUserID, msTeamsUserID)
);
