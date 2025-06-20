-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied

-- Add new fields to the imap table
ALTER TABLE imap
ADD COLUMN login_failures INT NOT NULL DEFAULT 0,
ADD COLUMN last_login_error TIMESTAMP NULL DEFAULT NULL,
ADD COLUMN name VARCHAR(255) NOT NULL DEFAULT 'Default IMAP Configuration';

-- Create non_campaign_reports table
CREATE TABLE IF NOT EXISTS non_campaign_reports (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    user_id BIGINT NOT NULL,
    imap_id BIGINT NOT NULL,
    reporter_email VARCHAR(255) NOT NULL,
    subject VARCHAR(255) NOT NULL,
    reported_at TIMESTAMP NOT NULL,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (imap_id) REFERENCES imap(id) ON DELETE CASCADE
);

-- Create non_campaign_stats table
CREATE TABLE IF NOT EXISTS non_campaign_stats (
    user_id BIGINT PRIMARY KEY,
    report_count INT NOT NULL DEFAULT 0,
    last_reported_at TIMESTAMP NULL DEFAULT NULL,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back

-- Drop the non_campaign_stats table
DROP TABLE IF EXISTS non_campaign_stats;

-- Drop the non_campaign_reports table
DROP TABLE IF EXISTS non_campaign_reports;

-- Remove the new columns from the imap table
ALTER TABLE imap
DROP COLUMN login_failures,
DROP COLUMN last_login_error,
DROP COLUMN name;
