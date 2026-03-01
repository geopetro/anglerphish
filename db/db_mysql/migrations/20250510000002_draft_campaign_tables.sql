-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied
CREATE TABLE draft_campaign_sets (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    user_id BIGINT,
    name VARCHAR(255) NOT NULL,
    created_date DATETIME,
    modified_date DATETIME,
    launch_date DATETIME,
    send_by_date DATETIME,
    url VARCHAR(255),
    urlparam VARCHAR(255),
    qrsize VARCHAR(255),
    basicauth BOOLEAN,
    page_id BIGINT,
    smtp_id BIGINT,
    sms_id BIGINT
);

CREATE TABLE draft_campaigns (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    user_id BIGINT,
    name VARCHAR(255) NOT NULL,
    created_date DATETIME,
    modified_date DATETIME,
    launch_date DATETIME,
    send_by_date DATETIME,
    draft_campaign_set_id BIGINT,
    template_id BIGINT,
    sms_template_id BIGINT,
    page_id BIGINT,
    smtp_id BIGINT,
    sms_id BIGINT,
    url VARCHAR(255),
    urlparam VARCHAR(255),
    qrsize VARCHAR(255),
    basicauth BOOLEAN,
    type VARCHAR(255) DEFAULT 'email'
);

CREATE TABLE draft_campaign_groups (
    draft_campaign_id BIGINT NOT NULL,
    group_id BIGINT NOT NULL,
    PRIMARY KEY (draft_campaign_id, group_id)
);

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back
DROP TABLE draft_campaign_groups;
DROP TABLE draft_campaigns;
DROP TABLE draft_campaign_sets;
