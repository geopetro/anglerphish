-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied
CREATE TABLE IF NOT EXISTS sms_logs (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    user_id BIGINT,
    campaign_id BIGINT,
    r_id VARCHAR(255),
    send_date DATETIME,
    send_attempt INT,
    processing BOOLEAN,
    target VARCHAR(255)
);

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back
DROP TABLE sms_logs;
