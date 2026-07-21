-- +goose Up
ALTER TABLE imap ADD COLUMN capture_reply_body BOOLEAN NOT NULL DEFAULT 1;

-- +goose Down
ALTER TABLE imap DROP COLUMN capture_reply_body;
