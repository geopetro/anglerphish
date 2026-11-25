-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied

-- Holds the single row of operator Direct Injection settings, edited from
-- Settings > Direct Injection. google_key is stored encrypted when an
-- encryption key is loaded.
CREATE TABLE IF NOT EXISTS dmi_settings (
  id BIGINT(20) NOT NULL AUTO_INCREMENT,
  google_key TEXT NOT NULL, -- MySQL doesn't allow a literal default on TEXT; the app always sets it
  updated_at DATETIME NOT NULL,
  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back
DROP TABLE IF EXISTS dmi_settings;
