-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied
ALTER TABLE results ADD COLUMN sms_target INTEGER DEFAULT 0;

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back
PRAGMA foreign_keys=off;

CREATE TABLE results_backup (
    id INTEGER PRIMARY KEY,
    campaign_id INTEGER,
    user_id INTEGER,
    r_id TEXT,
    status TEXT,
    ip TEXT,
    latitude REAL,
    longitude REAL,
    send_date TIMESTAMP,
    reported INTEGER,
    modified_date TIMESTAMP,
    email TEXT,
    first_name TEXT,
    last_name TEXT,
    position TEXT,
    custom TEXT
);

INSERT INTO results_backup SELECT id, campaign_id, user_id, r_id, status, ip, latitude, longitude, send_date, reported, modified_date, email, first_name, last_name, position, custom FROM results;
DROP TABLE results;

CREATE TABLE results (
    id INTEGER PRIMARY KEY,
    campaign_id INTEGER,
    user_id INTEGER,
    r_id TEXT,
    status TEXT,
    ip TEXT,
    latitude REAL,
    longitude REAL,
    send_date TIMESTAMP,
    reported INTEGER,
    modified_date TIMESTAMP,
    email TEXT,
    first_name TEXT,
    last_name TEXT,
    position TEXT,
    custom TEXT
);

INSERT INTO results SELECT id, campaign_id, user_id, r_id, status, ip, latitude, longitude, send_date, reported, modified_date, email, first_name, last_name, position, custom FROM results_backup;
DROP TABLE results_backup;

PRAGMA foreign_keys=on;
