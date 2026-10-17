-- +goose Up
ALTER TABLE draft_campaign_sets ADD COLUMN send_order VARCHAR(16) NOT NULL DEFAULT 'sequential';

-- +goose Down
ALTER TABLE draft_campaign_sets DROP COLUMN send_order;
