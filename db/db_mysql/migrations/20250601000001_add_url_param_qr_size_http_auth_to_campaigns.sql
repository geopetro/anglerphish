-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied
-- Check if columns exist before adding them
SET @query = IF(
    NOT EXISTS(
        SELECT * FROM INFORMATION_SCHEMA.COLUMNS 
        WHERE TABLE_SCHEMA = DATABASE() 
        AND TABLE_NAME = 'campaigns' 
        AND COLUMN_NAME = 'url_param'
    ),
    'ALTER TABLE `campaigns` ADD COLUMN `url_param` VARCHAR(255);',
    'SELECT 1'
);
PREPARE stmt FROM @query;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

SET @query = IF(
    NOT EXISTS(
        SELECT * FROM INFORMATION_SCHEMA.COLUMNS 
        WHERE TABLE_SCHEMA = DATABASE() 
        AND TABLE_NAME = 'campaigns' 
        AND COLUMN_NAME = 'qr_size'
    ),
    'ALTER TABLE `campaigns` ADD COLUMN `qr_size` VARCHAR(255);',
    'SELECT 1'
);
PREPARE stmt FROM @query;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

SET @query = IF(
    NOT EXISTS(
        SELECT * FROM INFORMATION_SCHEMA.COLUMNS 
        WHERE TABLE_SCHEMA = DATABASE() 
        AND TABLE_NAME = 'campaigns' 
        AND COLUMN_NAME = 'http_auth'
    ),
    'ALTER TABLE `campaigns` ADD COLUMN `http_auth` BOOLEAN;',
    'SELECT 1'
);
PREPARE stmt FROM @query;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back
ALTER TABLE `campaigns` DROP COLUMN IF EXISTS `url_param`;
ALTER TABLE `campaigns` DROP COLUMN IF EXISTS `qr_size`;
ALTER TABLE `campaigns` DROP COLUMN IF EXISTS `http_auth`;
