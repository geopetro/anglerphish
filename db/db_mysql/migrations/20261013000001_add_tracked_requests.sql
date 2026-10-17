-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied

-- Structured storage for X-Tracked requests (macro/POST callbacks that carry
-- no recipient id, so they cannot be attributed to a campaign result). This
-- replaces the legacy tracked-data.csv file.
CREATE TABLE IF NOT EXISTS tracked_requests (
  id BIGINT(20) NOT NULL AUTO_INCREMENT,
  created_at DATETIME NOT NULL,
  source_ip VARCHAR(255) NOT NULL DEFAULT '',
  method VARCHAR(16) NOT NULL DEFAULT '',
  path VARCHAR(2048) NOT NULL DEFAULT '',
  raw_query TEXT NOT NULL,
  user_agent VARCHAR(1024) NOT NULL DEFAULT '',
  PRIMARY KEY (id),
  INDEX (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back
DROP TABLE IF EXISTS tracked_requests;
