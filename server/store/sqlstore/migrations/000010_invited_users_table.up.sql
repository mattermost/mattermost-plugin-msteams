CREATE TABLE IF NOT EXISTS msteamssync_invited_users (
    mmUserID VARCHAR(255) PRIMARY KEY
);

ALTER TABLE msteamssync_invited_users ADD COLUMN IF NOT EXISTS invitePendingSince BIGINT;
ALTER TABLE msteamssync_invited_users ADD COLUMN IF NOT EXISTS inviteLastSentAt BIGINT;
