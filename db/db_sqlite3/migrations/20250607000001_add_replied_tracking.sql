-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied
ALTER TABLE results ADD COLUMN replied BOOLEAN NOT NULL DEFAULT 0;
ALTER TABLE imap ADD COLUMN tracking_type INTEGER NOT NULL DEFAULT 0;

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back
ALTER TABLE results DROP COLUMN replied;
ALTER TABLE imap DROP COLUMN tracking_type;
