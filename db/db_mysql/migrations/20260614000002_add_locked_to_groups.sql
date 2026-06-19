-- +goose Up
ALTER TABLE groups ADD COLUMN locked BOOLEAN NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE groups DROP COLUMN locked;
