-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied
CREATE TABLE draft_campaign_sets (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER,
    name TEXT NOT NULL,
    created_date TIMESTAMP,
    modified_date TIMESTAMP,
    launch_date TIMESTAMP,
    send_by_date TIMESTAMP,
    url TEXT,
    urlparam TEXT,
    qrsize TEXT,
    basicauth BOOLEAN,
    page_id INTEGER,
    smtp_id INTEGER,
    sms_id INTEGER
);

CREATE TABLE draft_campaigns (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER,
    name TEXT NOT NULL,
    created_date TIMESTAMP,
    modified_date TIMESTAMP,
    launch_date TIMESTAMP,
    send_by_date TIMESTAMP,
    draft_campaign_set_id INTEGER,
    template_id INTEGER,
    sms_template_id INTEGER,
    page_id INTEGER,
    smtp_id INTEGER,
    sms_id INTEGER,
    url TEXT,
    urlparam TEXT,
    qrsize TEXT,
    basicauth BOOLEAN,
    type TEXT DEFAULT 'email'
);

CREATE TABLE draft_campaign_groups (
    draft_campaign_id INTEGER NOT NULL,
    group_id INTEGER NOT NULL,
    PRIMARY KEY (draft_campaign_id, group_id)
);

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back
DROP TABLE draft_campaign_groups;
DROP TABLE draft_campaigns;
DROP TABLE draft_campaign_sets;
