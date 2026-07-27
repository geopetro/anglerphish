-- +goose Up
ALTER TABLE imap ADD COLUMN capture_reply_body BOOLEAN NOT NULL DEFAULT 1;

-- +goose Down
-- SQLite does not support DROP COLUMN in older versions; migration is intentionally left empty
