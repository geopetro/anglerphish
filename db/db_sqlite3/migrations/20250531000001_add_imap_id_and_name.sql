-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied

-- First create a new table with the desired structure
CREATE TABLE imap_new (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL DEFAULT 'Default IMAP Configuration',
  user_id INTEGER,
  enabled INTEGER,
  host TEXT,
  port INTEGER,
  username TEXT,
  password TEXT,
  tls INTEGER,
  ignore_cert_errors INTEGER,
  folder TEXT,
  restrict_domain TEXT,
  delete_reported_campaign_email INTEGER,
  last_login TIMESTAMP,
  login_failures INTEGER,
  last_login_error TIMESTAMP,
  modified_date TIMESTAMP,
  imap_freq INTEGER
);

-- Copy data from the old table to the new one
INSERT INTO imap_new (
  user_id, enabled, host, port, username, password, tls, ignore_cert_errors,
  folder, restrict_domain, delete_reported_campaign_email, last_login, 
  login_failures, last_login_error, modified_date, imap_freq
)
SELECT 
  user_id, enabled, host, port, username, password, tls, ignore_cert_errors,
  folder, restrict_domain, delete_reported_campaign_email, last_login, 
  login_failures, last_login_error, modified_date, imap_freq
FROM imap;

-- Ensure each user's existing configuration gets a unique name
UPDATE imap_new SET name = 'IMAP Configuration for User ' || user_id;

-- Drop the old table
DROP TABLE imap;

-- Rename the new table to the original name
ALTER TABLE imap_new RENAME TO imap;

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back

-- Create a temporary table without id and name columns
CREATE TABLE imap_temp (
  user_id INTEGER PRIMARY KEY,
  enabled INTEGER,
  host TEXT,
  port INTEGER,
  username TEXT,
  password TEXT,
  tls INTEGER,
  ignore_cert_errors INTEGER,
  folder TEXT,
  restrict_domain TEXT,
  delete_reported_campaign_email INTEGER,
  last_login TIMESTAMP,
  login_failures INTEGER,
  last_login_error TIMESTAMP,
  modified_date TIMESTAMP,
  imap_freq INTEGER
);

-- Copy data without the new columns
INSERT INTO imap_temp (
  user_id, enabled, host, port, username, password, tls, ignore_cert_errors,
  folder, restrict_domain, delete_reported_campaign_email, last_login, 
  login_failures, last_login_error, modified_date, imap_freq
)
SELECT 
  user_id, enabled, host, port, username, password, tls, ignore_cert_errors,
  folder, restrict_domain, delete_reported_campaign_email, last_login, 
  login_failures, last_login_error, modified_date, imap_freq
FROM imap;

-- Drop the modified table
DROP TABLE imap;

-- Rename temp table back to original
ALTER TABLE imap_temp RENAME TO imap;
