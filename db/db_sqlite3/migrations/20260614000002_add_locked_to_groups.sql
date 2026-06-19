-- +goose Up
ALTER TABLE groups ADD COLUMN locked BOOLEAN NOT NULL DEFAULT 0;

-- +goose Down
-- SQLite does not support DROP COLUMN in older versions; migration is intentionally left empty
