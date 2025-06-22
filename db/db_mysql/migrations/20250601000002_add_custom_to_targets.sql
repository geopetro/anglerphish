-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied
ALTER TABLE `targets` ADD COLUMN `custom` varchar(255);

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back
ALTER TABLE `targets` DROP COLUMN `custom`;
