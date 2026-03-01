-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied
-- Check if column exists before adding it
SET @query = IF(
    NOT EXISTS(
        SELECT * FROM INFORMATION_SCHEMA.COLUMNS 
        WHERE TABLE_SCHEMA = DATABASE() 
        AND TABLE_NAME = 'results' 
        AND COLUMN_NAME = 'custom'
    ),
    'ALTER TABLE `results` ADD COLUMN `custom` VARCHAR(255);',
    'SELECT 1'
);
PREPARE stmt FROM @query;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back
ALTER TABLE `results` DROP COLUMN IF EXISTS `custom`;
