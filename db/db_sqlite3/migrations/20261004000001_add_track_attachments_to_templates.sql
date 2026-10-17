-- +goose Up
ALTER TABLE templates ADD COLUMN track_attachments BOOLEAN NOT NULL DEFAULT 0;

-- +goose Down
-- SQLite does not support DROP COLUMN in older versions; migration is intentionally left empty
