-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied
ALTER TABLE campaigns ADD COLUMN url_param varchar(255);
ALTER TABLE campaigns ADD COLUMN qr_size varchar(255);
ALTER TABLE campaigns ADD COLUMN http_auth integer;

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back
-- SQLite doesn't support dropping columns directly, so we need to recreate the table
CREATE TABLE campaigns_temp (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER,
    name TEXT NOT NULL,
    created_date TIMESTAMP,
    completed_date TIMESTAMP,
    template_id INTEGER,
    page_id INTEGER,
    status TEXT,
    url TEXT,
    campaign_set_id INTEGER,
    type TEXT DEFAULT 'email',
    sms_id INTEGER,
    sms_template_id INTEGER,
    send_by_date TIMESTAMP,
    smtp_id INTEGER,
    launch_date TIMESTAMP
);

INSERT INTO campaigns_temp SELECT id, user_id, name, created_date, completed_date, template_id, page_id, status, url, campaign_set_id, type, sms_id, sms_template_id, send_by_date, smtp_id, launch_date FROM campaigns;

DROP TABLE campaigns;

ALTER TABLE campaigns_temp RENAME TO campaigns;
