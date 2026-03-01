-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied
ALTER TABLE results ADD COLUMN custom VARCHAR(255);

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back
-- SQLite doesn't support dropping columns directly, so we need to recreate the table
CREATE TABLE results_temp (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    campaign_id INTEGER,
    user_id INTEGER,
    r_id TEXT,
    email TEXT,
    first_name TEXT,
    last_name TEXT,
    status TEXT NOT NULL,
    ip TEXT,
    latitude REAL,
    longitude REAL,
    position TEXT
);

INSERT INTO results_temp SELECT id, campaign_id, user_id, r_id, email, first_name, last_name, status, ip, latitude, longitude, position FROM results;

DROP TABLE results;

ALTER TABLE results_temp RENAME TO results;
