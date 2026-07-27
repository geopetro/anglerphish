-- +goose Up
ALTER TABLE non_campaign_reports ADD COLUMN imap_uid BIGINT NOT NULL DEFAULT 0;
ALTER TABLE non_campaign_reports ADD COLUMN imap_uidvalidity BIGINT NOT NULL DEFAULT 0;
ALTER TABLE non_campaign_reports ADD COLUMN message_id VARCHAR(255) NOT NULL DEFAULT '';

-- +goose Down
-- SQLite does not support DROP COLUMN in older versions; migration is intentionally left empty
