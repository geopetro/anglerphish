-- +goose Up
-- `groups` is a reserved word in MySQL 8, so it must be quoted.
ALTER TABLE `groups` ADD COLUMN locked BOOLEAN NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE `groups` DROP COLUMN locked;
