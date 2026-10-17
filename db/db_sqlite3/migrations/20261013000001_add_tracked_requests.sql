-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied

-- Structured storage for X-Tracked requests (macro/POST callbacks that carry
-- no recipient id, so they cannot be attributed to a campaign result). This
-- replaces the legacy tracked-data.csv file.
CREATE TABLE IF NOT EXISTS tracked_requests (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  created_at TIMESTAMP NOT NULL,
  source_ip TEXT NOT NULL DEFAULT '',
  method TEXT NOT NULL DEFAULT '',
  path TEXT NOT NULL DEFAULT '',
  raw_query TEXT NOT NULL DEFAULT '',
  user_agent TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_tracked_requests_created_at ON tracked_requests(created_at);

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back
DROP TABLE IF EXISTS tracked_requests;
