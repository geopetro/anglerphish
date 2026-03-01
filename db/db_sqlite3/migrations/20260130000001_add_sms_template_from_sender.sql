-- +goose Up
-- Add optional from_sender field to sms_templates table
ALTER TABLE sms_templates ADD COLUMN from_sender VARCHAR(255) DEFAULT '';

-- +goose Down
-- SQLite doesn't support DROP COLUMN directly, but goose handles this
-- For SQLite, we would need to recreate the table without the column
-- This is a simplified down migration
