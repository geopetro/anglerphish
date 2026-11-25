-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied

-- Direct Mail Injection settings for sending profiles. dmi_secret is stored
-- encrypted when an encryption key is loaded.
ALTER TABLE smtp ADD COLUMN dmi_auth_method VARCHAR(32) NOT NULL DEFAULT '';
ALTER TABLE smtp ADD COLUMN dmi_tenant_id VARCHAR(255) NOT NULL DEFAULT '';
ALTER TABLE smtp ADD COLUMN dmi_client_id VARCHAR(255) NOT NULL DEFAULT '';
ALTER TABLE smtp ADD COLUMN dmi_secret TEXT NOT NULL DEFAULT '';
ALTER TABLE smtp ADD COLUMN dmi_certificate TEXT NOT NULL DEFAULT '';
ALTER TABLE smtp ADD COLUMN dmi_allowed_domains TEXT NOT NULL DEFAULT '';
ALTER TABLE smtp ADD COLUMN dmi_folder VARCHAR(32) NOT NULL DEFAULT '';
ALTER TABLE smtp ADD COLUMN dmi_folder_name VARCHAR(255) NOT NULL DEFAULT '';

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back
-- The bundled SQLite (3.31) cannot DROP COLUMN, so the columns are left in place
