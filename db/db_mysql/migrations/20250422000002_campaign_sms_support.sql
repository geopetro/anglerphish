-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied
ALTER TABLE campaigns ADD COLUMN type VARCHAR(10) DEFAULT 'email';
ALTER TABLE campaigns ADD COLUMN sms_id BIGINT;
ALTER TABLE campaigns ADD COLUMN sms_template_id BIGINT;

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back
ALTER TABLE campaigns DROP COLUMN type;
ALTER TABLE campaigns DROP COLUMN sms_id;
ALTER TABLE campaigns DROP COLUMN sms_template_id;
