-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied

-- Direct Mail Injection settings for sending profiles. dmi_secret is stored
-- encrypted when an encryption key is loaded. MySQL doesn't allow a literal
-- default on TEXT. Existing rows get '' and the app always sets them.
ALTER TABLE smtp
  ADD COLUMN dmi_auth_method VARCHAR(32) NOT NULL DEFAULT '',
  ADD COLUMN dmi_tenant_id VARCHAR(255) NOT NULL DEFAULT '',
  ADD COLUMN dmi_client_id VARCHAR(255) NOT NULL DEFAULT '',
  ADD COLUMN dmi_secret TEXT NOT NULL,
  ADD COLUMN dmi_certificate TEXT NOT NULL,
  ADD COLUMN dmi_allowed_domains TEXT NOT NULL,
  ADD COLUMN dmi_folder VARCHAR(32) NOT NULL DEFAULT '',
  ADD COLUMN dmi_folder_name VARCHAR(255) NOT NULL DEFAULT '';

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back
ALTER TABLE smtp
  DROP COLUMN dmi_auth_method,
  DROP COLUMN dmi_tenant_id,
  DROP COLUMN dmi_client_id,
  DROP COLUMN dmi_secret,
  DROP COLUMN dmi_certificate,
  DROP COLUMN dmi_allowed_domains,
  DROP COLUMN dmi_folder,
  DROP COLUMN dmi_folder_name;
