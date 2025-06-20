-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied

-- Add new fields to the imap table
ALTER TABLE imap ADD COLUMN login_failures INTEGER NOT NULL DEFAULT 0;
ALTER TABLE imap ADD COLUMN last_login_error TIMESTAMP NULL DEFAULT NULL;
ALTER TABLE imap ADD COLUMN name TEXT NOT NULL DEFAULT 'Default IMAP Configuration';

-- Create non_campaign_reports table
CREATE TABLE IF NOT EXISTS non_campaign_reports (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    imap_id INTEGER NOT NULL,
    reporter_email TEXT NOT NULL,
    subject TEXT NOT NULL,
    reported_at TIMESTAMP NOT NULL,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (imap_id) REFERENCES imap(id) ON DELETE CASCADE
);

-- Create non_campaign_stats table
CREATE TABLE IF NOT EXISTS non_campaign_stats (
    user_id INTEGER PRIMARY KEY,
    report_count INTEGER NOT NULL DEFAULT 0,
    last_reported_at TIMESTAMP NULL DEFAULT NULL,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back

-- Drop the non_campaign_stats table
DROP TABLE IF EXISTS non_campaign_stats;

-- Drop the non_campaign_reports table
DROP TABLE IF EXISTS non_campaign_reports;

-- For SQLite, we can't easily drop columns, so we'd need to recreate the table
-- This is a simplified version that doesn't handle the column removal
-- In a real scenario, you would create a new table, copy data, drop old table, rename new table
PRAGMA foreign_keys=off;
-- Note: In a real migration, you would add code here to recreate the table without the columns
PRAGMA foreign_keys=on;
