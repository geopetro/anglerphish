-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied
CREATE TABLE IF NOT EXISTS `sms_requests` (
    `id` bigint(20) NOT NULL AUTO_INCREMENT,
    `user_id` bigint(20),
    `sms_template_id` bigint(20),
    `page_id` bigint(20),
    `sms_id` bigint(20),
    `url` varchar(255),
    `r_id` varchar(255),
    `first_name` varchar(255),
    `last_name` varchar(255),
    `email` varchar(255),
    `position` varchar(255),
    `custom` varchar(255),
    PRIMARY KEY (`id`)
);

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back
DROP TABLE `sms_requests`;
