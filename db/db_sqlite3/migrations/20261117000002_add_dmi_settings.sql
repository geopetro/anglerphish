-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied

-- Holds the single row of operator Direct Injection settings, edited from
-- Settings > Direct Injection. google_key is stored encrypted when an
-- encryption key is loaded.
CREATE TABLE IF NOT EXISTS dmi_settings (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  google_key TEXT NOT NULL DEFAULT '',
  updated_at TIMESTAMP NOT NULL
);

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back
DROP TABLE IF EXISTS dmi_settings;
