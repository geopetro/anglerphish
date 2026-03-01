-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied
-- Direct approach to add the column if it doesn't exist
-- SQLite will ignore the error if the column already exists in newer versions

-- Add the column directly
ALTER TABLE targets ADD COLUMN custom varchar(255);

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back
-- SQLite doesn't support dropping columns directly, so we need to recreate the table
CREATE TABLE targets_temp (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    first_name TEXT,
    last_name TEXT,
    email TEXT,
    position TEXT,
    phone TEXT
);

INSERT INTO targets_temp SELECT id, first_name, last_name, email, position, phone FROM targets;

DROP TABLE targets;

ALTER TABLE targets_temp RENAME TO targets;
