-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied

-- First add the new id and name columns
ALTER TABLE imap ADD COLUMN id INT AUTO_INCREMENT PRIMARY KEY FIRST;
ALTER TABLE imap ADD COLUMN name VARCHAR(255) NOT NULL DEFAULT 'Default IMAP Configuration' AFTER id;

-- Ensure each user's existing configuration gets a unique name
UPDATE imap SET name = CONCAT('IMAP Configuration for User ', user_id) WHERE 1=1;

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back

-- Remove the new columns
ALTER TABLE imap DROP COLUMN id;
ALTER TABLE imap DROP COLUMN name;
