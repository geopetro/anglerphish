-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied
-- Check if column exists before adding it
SET @query = IF(
    NOT EXISTS(
        SELECT * FROM INFORMATION_SCHEMA.COLUMNS 
        WHERE TABLE_SCHEMA = DATABASE() 
        AND TABLE_NAME = 'email_requests' 
        AND COLUMN_NAME = 'phone'
    ),
    'ALTER TABLE `email_requests` ADD COLUMN `phone` VARCHAR(255);',
    'SELECT 1'
);
PREPARE stmt FROM @query;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back
ALTER TABLE `email_requests` DROP COLUMN IF EXISTS `phone`;
