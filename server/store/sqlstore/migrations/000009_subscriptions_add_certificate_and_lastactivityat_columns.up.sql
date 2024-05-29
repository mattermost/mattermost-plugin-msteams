ALTER TABLE msteamssync_subscriptions ADD COLUMN IF NOT EXISTS certificate TEXT;
ALTER TABLE msteamssync_subscriptions ADD COLUMN IF NOT EXISTS lastActivityAt BIGINT;
