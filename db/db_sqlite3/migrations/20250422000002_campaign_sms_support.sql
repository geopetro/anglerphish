-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied
ALTER TABLE campaigns ADD COLUMN type TEXT DEFAULT 'email';
ALTER TABLE campaigns ADD COLUMN sms_id INTEGER;
ALTER TABLE campaigns ADD COLUMN sms_template_id INTEGER;

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back
PRAGMA foreign_keys=off;

CREATE TABLE campaigns_backup (
    id INTEGER PRIMARY KEY,
    user_id INTEGER,
    name TEXT,
    created_date TIMESTAMP,
    launch_date TIMESTAMP,
    send_by_date TIMESTAMP,
    completed_date TIMESTAMP,
    template_id INTEGER,
    page_id INTEGER,
    status TEXT,
    smtp_id INTEGER,
    url TEXT,
    urlparam TEXT,
    qrsize TEXT,
    basicauth INTEGER
);

INSERT INTO campaigns_backup SELECT id, user_id, name, created_date, launch_date, send_by_date, completed_date, template_id, page_id, status, smtp_id, url, urlparam, qrsize, basicauth FROM campaigns;
DROP TABLE campaigns;

CREATE TABLE campaigns (
    id INTEGER PRIMARY KEY,
    user_id INTEGER,
    name TEXT,
    created_date TIMESTAMP,
    launch_date TIMESTAMP,
    send_by_date TIMESTAMP,
    completed_date TIMESTAMP,
    template_id INTEGER,
    page_id INTEGER,
    status TEXT,
    smtp_id INTEGER,
    url TEXT,
    urlparam TEXT,
    qrsize TEXT,
    basicauth INTEGER
);

INSERT INTO campaigns SELECT id, user_id, name, created_date, launch_date, send_by_date, completed_date, template_id, page_id, status, smtp_id, url, urlparam, qrsize, basicauth FROM campaigns_backup;
DROP TABLE campaigns_backup;

PRAGMA foreign_keys=on;
