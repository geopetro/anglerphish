-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied
ALTER TABLE imap ADD COLUMN login_failures INTEGER NOT NULL DEFAULT 0;
ALTER TABLE imap ADD COLUMN last_login_error TIMESTAMP DEFAULT NULL;

-- Create non_campaign_reports table
CREATE TABLE IF NOT EXISTS non_campaign_reports (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id INTEGER NOT NULL,
  imap_id INTEGER NOT NULL,
  reporter_email TEXT NOT NULL,
  subject TEXT NOT NULL,
  reported_at TIMESTAMP NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_non_campaign_reports_user_id ON non_campaign_reports(user_id);
CREATE INDEX IF NOT EXISTS idx_non_campaign_reports_reported_at ON non_campaign_reports(reported_at);

-- Create non_campaign_stats table
CREATE TABLE IF NOT EXISTS non_campaign_stats (
  user_id INTEGER PRIMARY KEY,
  report_count INTEGER NOT NULL DEFAULT 0,
  last_reported_at TIMESTAMP DEFAULT NULL
);

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back

-- Create a temporary table
CREATE TABLE imap_temp (
  user_id INTEGER PRIMARY KEY,
  enabled INTEGER NOT NULL,
  host TEXT NOT NULL,
  port INTEGER NOT NULL,
  username TEXT NOT NULL,
  password TEXT NOT NULL,
  tls INTEGER NOT NULL,
  ignore_cert_errors INTEGER NOT NULL,
  folder TEXT NOT NULL,
  restrict_domain TEXT NOT NULL,
  delete_reported_campaign_email INTEGER NOT NULL,
  last_login TIMESTAMP,
  modified_date TIMESTAMP,
  imap_freq INTEGER NOT NULL
);

-- Copy data to the temporary table
INSERT INTO imap_temp 
SELECT user_id, enabled, host, port, username, password, tls, ignore_cert_errors, folder, restrict_domain, delete_reported_campaign_email, last_login, modified_date, imap_freq
FROM imap;

-- Drop the original table
DROP TABLE imap;

-- Recreate the original table without the new columns
CREATE TABLE imap (
  user_id INTEGER PRIMARY KEY,
  enabled INTEGER NOT NULL,
  host TEXT NOT NULL,
  port INTEGER NOT NULL,
  username TEXT NOT NULL,
  password TEXT NOT NULL,
  tls INTEGER NOT NULL,
  ignore_cert_errors INTEGER NOT NULL,
  folder TEXT NOT NULL,
  restrict_domain TEXT NOT NULL,
  delete_reported_campaign_email INTEGER NOT NULL,
  last_login TIMESTAMP,
  modified_date TIMESTAMP,
  imap_freq INTEGER NOT NULL
);

-- Copy the data back
INSERT INTO imap 
SELECT user_id, enabled, host, port, username, password, tls, ignore_cert_errors, folder, restrict_domain, delete_reported_campaign_email, last_login, modified_date, imap_freq
FROM imap_temp;

-- Drop the temporary table
DROP TABLE imap_temp;

-- Drop the new tables
DROP TABLE IF EXISTS non_campaign_reports;
DROP TABLE IF EXISTS non_campaign_stats;
