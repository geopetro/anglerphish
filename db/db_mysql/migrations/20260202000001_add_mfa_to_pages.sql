-- +goose Up
-- Add MFA fields to pages table
ALTER TABLE pages ADD COLUMN enable_mfa BOOLEAN DEFAULT FALSE;
ALTER TABLE pages ADD COLUMN mfa_sms_profile_id BIGINT DEFAULT 0;
ALTER TABLE pages ADD COLUMN mfa_from VARCHAR(255) DEFAULT '';
ALTER TABLE pages ADD COLUMN mfa_message TEXT;
ALTER TABLE pages ADD COLUMN mfa_code_length INT DEFAULT 6;
ALTER TABLE pages ADD COLUMN mfa_code_type VARCHAR(20) DEFAULT 'numeric';
ALTER TABLE pages ADD COLUMN mfa_inject_page BOOLEAN DEFAULT TRUE;
ALTER TABLE pages ADD COLUMN mfa_page_html TEXT;

-- Create mfa_codes table for storing generated MFA codes
CREATE TABLE IF NOT EXISTS mfa_codes (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    r_id VARCHAR(255) NOT NULL,
    code VARCHAR(20) NOT NULL,
    phone VARCHAR(50) NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    verified BOOLEAN DEFAULT FALSE,
    INDEX idx_mfa_codes_rid (r_id)
);

-- +goose Down
ALTER TABLE pages DROP COLUMN enable_mfa;
ALTER TABLE pages DROP COLUMN mfa_sms_profile_id;
ALTER TABLE pages DROP COLUMN mfa_from;
ALTER TABLE pages DROP COLUMN mfa_message;
ALTER TABLE pages DROP COLUMN mfa_code_length;
ALTER TABLE pages DROP COLUMN mfa_code_type;
ALTER TABLE pages DROP COLUMN mfa_inject_page;
ALTER TABLE pages DROP COLUMN mfa_page_html;
DROP TABLE IF EXISTS mfa_codes;
