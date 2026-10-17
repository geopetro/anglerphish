-- +goose Up
ALTER TABLE draft_campaigns ADD COLUMN randomize_send_order BOOLEAN NOT NULL DEFAULT 0;

-- +goose Down
-- SQLite does not support DROP COLUMN in older versions; migration is intentionally left empty
