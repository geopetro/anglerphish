-- +goose Up
-- SQL in this section is executed when the migration is applied.
ALTER TABLE campaign_sets ADD COLUMN use_shared_page BOOLEAN DEFAULT TRUE;
ALTER TABLE campaign_sets ADD COLUMN use_shared_url BOOLEAN DEFAULT TRUE;
ALTER TABLE campaign_sets ADD COLUMN use_shared_urlparam BOOLEAN DEFAULT TRUE;
ALTER TABLE campaign_sets ADD COLUMN use_shared_qrsize BOOLEAN DEFAULT TRUE;
ALTER TABLE campaign_sets ADD COLUMN use_shared_httpauth BOOLEAN DEFAULT TRUE;
ALTER TABLE campaign_sets ADD COLUMN use_shared_schedule BOOLEAN DEFAULT TRUE;

ALTER TABLE draft_campaign_sets ADD COLUMN use_shared_page BOOLEAN DEFAULT TRUE;
ALTER TABLE draft_campaign_sets ADD COLUMN use_shared_url BOOLEAN DEFAULT TRUE;
ALTER TABLE draft_campaign_sets ADD COLUMN use_shared_urlparam BOOLEAN DEFAULT TRUE;
ALTER TABLE draft_campaign_sets ADD COLUMN use_shared_qrsize BOOLEAN DEFAULT TRUE;
ALTER TABLE draft_campaign_sets ADD COLUMN use_shared_httpauth BOOLEAN DEFAULT TRUE;
ALTER TABLE draft_campaign_sets ADD COLUMN use_shared_schedule BOOLEAN DEFAULT TRUE;

-- +goose Down
-- SQL in this section is executed when the migration is rolled back.
ALTER TABLE campaign_sets DROP COLUMN use_shared_page;
ALTER TABLE campaign_sets DROP COLUMN use_shared_url;
ALTER TABLE campaign_sets DROP COLUMN use_shared_urlparam;
ALTER TABLE campaign_sets DROP COLUMN use_shared_qrsize;
ALTER TABLE campaign_sets DROP COLUMN use_shared_httpauth;
ALTER TABLE campaign_sets DROP COLUMN use_shared_schedule;

ALTER TABLE draft_campaign_sets DROP COLUMN use_shared_page;
ALTER TABLE draft_campaign_sets DROP COLUMN use_shared_url;
ALTER TABLE draft_campaign_sets DROP COLUMN use_shared_urlparam;
ALTER TABLE draft_campaign_sets DROP COLUMN use_shared_qrsize;
ALTER TABLE draft_campaign_sets DROP COLUMN use_shared_httpauth;
ALTER TABLE draft_campaign_sets DROP COLUMN use_shared_schedule;
