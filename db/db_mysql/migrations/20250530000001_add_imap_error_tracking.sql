-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied
ALTER TABLE imap 
  ADD COLUMN login_failures INT NOT NULL DEFAULT 0,
  ADD COLUMN last_login_error DATETIME DEFAULT NULL;

-- Create non_campaign_reports table
CREATE TABLE IF NOT EXISTS non_campaign_reports (
  id BIGINT(20) NOT NULL AUTO_INCREMENT,
  user_id BIGINT(20) NOT NULL,
  imap_id BIGINT(20) NOT NULL,
  reporter_email VARCHAR(255) NOT NULL,
  subject VARCHAR(255) NOT NULL,
  reported_at DATETIME NOT NULL,
  PRIMARY KEY (id),
  INDEX (user_id),
  INDEX (reported_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Create non_campaign_stats table
CREATE TABLE IF NOT EXISTS non_campaign_stats (
  user_id BIGINT(20) NOT NULL,
  report_count INT NOT NULL DEFAULT 0,
  last_reported_at DATETIME DEFAULT NULL,
  PRIMARY KEY (user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back
ALTER TABLE imap 
  DROP COLUMN login_failures,
  DROP COLUMN last_login_error;

DROP TABLE IF EXISTS non_campaign_reports;
DROP TABLE IF EXISTS non_campaign_stats;
