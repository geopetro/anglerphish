-- +goose Up
ALTER TABLE campaign_sets ADD COLUMN send_order VARCHAR(16) NOT NULL DEFAULT 'sequential';

-- +goose Down
-- SQLite does not support DROP COLUMN in older versions; migration is intentionally left empty
