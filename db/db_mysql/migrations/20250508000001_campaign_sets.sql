-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied
CREATE TABLE campaign_sets (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    user_id BIGINT,
    name VARCHAR(255) NOT NULL,
    created_date DATETIME,
    launch_date DATETIME,
    send_by_date DATETIME,
    completed_date DATETIME,
    status VARCHAR(255),
    url VARCHAR(255),
    urlparam VARCHAR(255),
    qrsize VARCHAR(255),
    basicauth BOOLEAN,
    page_id BIGINT,
    smtp_id BIGINT,
    sms_id BIGINT
);

-- Add campaign_set_id column to campaigns table
ALTER TABLE campaigns ADD COLUMN campaign_set_id BIGINT;

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back
DROP TABLE campaign_sets;
ALTER TABLE campaigns DROP COLUMN campaign_set_id;
