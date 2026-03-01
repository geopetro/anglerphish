-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied
CREATE TABLE IF NOT EXISTS `sms_profiles` (
    `id` bigint(20) NOT NULL AUTO_INCREMENT,
    `user_id` bigint(20),
    `name` varchar(255),
    `provider` varchar(50),
    `from` varchar(50),
    `provider_config` text,
    `modified_date` datetime,
    PRIMARY KEY (`id`)
);


-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back
DROP TABLE `sms_profiles`;
