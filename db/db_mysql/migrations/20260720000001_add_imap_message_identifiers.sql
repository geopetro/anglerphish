-- +goose Up
ALTER TABLE non_campaign_reports ADD COLUMN imap_uid BIGINT NOT NULL DEFAULT 0;
ALTER TABLE non_campaign_reports ADD COLUMN imap_uidvalidity BIGINT NOT NULL DEFAULT 0;
ALTER TABLE non_campaign_reports ADD COLUMN message_id VARCHAR(255) NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE non_campaign_reports DROP COLUMN imap_uid;
ALTER TABLE non_campaign_reports DROP COLUMN imap_uidvalidity;
ALTER TABLE non_campaign_reports DROP COLUMN message_id;
