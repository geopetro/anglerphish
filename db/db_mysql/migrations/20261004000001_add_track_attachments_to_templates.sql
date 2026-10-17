-- +goose Up
ALTER TABLE templates ADD COLUMN track_attachments BOOLEAN NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE templates DROP COLUMN track_attachments;
