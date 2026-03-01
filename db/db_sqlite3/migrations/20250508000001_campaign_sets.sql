-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied
CREATE TABLE campaign_sets (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER,
    name TEXT NOT NULL,
    created_date TIMESTAMP,
    launch_date TIMESTAMP,
    send_by_date TIMESTAMP,
    completed_date TIMESTAMP,
    status TEXT,
    url TEXT,
    urlparam TEXT,
    qrsize TEXT,
    basicauth BOOLEAN,
    page_id INTEGER,
    smtp_id INTEGER,
    sms_id INTEGER
);

-- Add campaign_set_id column to campaigns table
ALTER TABLE campaigns ADD COLUMN campaign_set_id INTEGER;

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back
DROP TABLE campaign_sets;
ALTER TABLE campaigns DROP COLUMN campaign_set_id;
