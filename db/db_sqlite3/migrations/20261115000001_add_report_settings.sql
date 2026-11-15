-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied

-- Holds the single row of report generation settings, edited from
-- Settings > Report Generation. Defaults apply until a row is saved.
CREATE TABLE IF NOT EXISTS report_settings (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  online_geolocation BOOLEAN NOT NULL DEFAULT 0,
  geoip_db_path VARCHAR(1024) NOT NULL DEFAULT '',
  job_timeout_minutes INTEGER NOT NULL DEFAULT 30,
  retention_days INTEGER NOT NULL DEFAULT 30,
  updated_at TIMESTAMP NOT NULL
);

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back
DROP TABLE IF EXISTS report_settings;
