-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied
ALTER TABLE draft_campaign_sets ADD COLUMN use_shared_settings BOOLEAN DEFAULT 1;

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back
-- SQLite doesn't support dropping columns directly, so we need to recreate the table
CREATE TABLE draft_campaign_sets_temp (
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

INSERT INTO draft_campaign_sets_temp SELECT id, user_id, name, created_date, modified_date, launch_date, send_by_date, url, urlparam, qrsize, basicauth, page_id, smtp_id, sms_id FROM draft_campaign_sets;

DROP TABLE draft_campaign_sets;

ALTER TABLE draft_campaign_sets_temp RENAME TO draft_campaign_sets;
