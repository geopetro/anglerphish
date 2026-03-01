-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied
CREATE TABLE IF NOT EXISTS sms_templates (
    id BIGINT(20) NOT NULL AUTO_INCREMENT,
    user_id BIGINT(20),
    name VARCHAR(255),
    text TEXT,
    char_count INT,
    modified_date DATETIME,
    PRIMARY KEY (id)
);

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back
DROP TABLE sms_templates;
