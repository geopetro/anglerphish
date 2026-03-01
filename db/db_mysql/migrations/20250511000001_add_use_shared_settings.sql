-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied
ALTER TABLE draft_campaign_sets ADD COLUMN use_shared_settings BOOLEAN DEFAULT TRUE;

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back
ALTER TABLE draft_campaign_sets DROP COLUMN use_shared_settings;
