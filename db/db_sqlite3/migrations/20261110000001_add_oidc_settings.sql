-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied

-- Holds the single row of admin SSO (OIDC) settings. The client secret is
-- stored encrypted when an encryption key is loaded.
CREATE TABLE IF NOT EXISTS oidc_settings (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  enabled BOOLEAN NOT NULL DEFAULT 0,
  provider VARCHAR(32) NOT NULL DEFAULT '',
  issuer VARCHAR(255) NOT NULL DEFAULT '',
  client_id VARCHAR(255) NOT NULL DEFAULT '',
  client_secret TEXT NOT NULL DEFAULT '',
  redirect_url VARCHAR(255) NOT NULL DEFAULT '',
  required_group VARCHAR(255) NOT NULL DEFAULT '',
  groups_claim VARCHAR(255) NOT NULL DEFAULT '',
  username_from_email VARCHAR(32) NOT NULL DEFAULT '',
  allow_unverified_email BOOLEAN NOT NULL DEFAULT 0,
  updated_at TIMESTAMP NOT NULL
);

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back
DROP TABLE IF EXISTS oidc_settings;
