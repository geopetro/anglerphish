-- +goose Up
-- Add optional from_sender field to sms_templates table
ALTER TABLE sms_templates ADD COLUMN from_sender VARCHAR(255) DEFAULT '';

-- +goose Down
ALTER TABLE sms_templates DROP COLUMN from_sender;
