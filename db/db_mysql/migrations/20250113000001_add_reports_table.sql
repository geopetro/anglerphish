-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied

CREATE TABLE IF NOT EXISTS reports (
    id BIGINT NOT NULL AUTO_INCREMENT,
    user_id BIGINT NOT NULL,
    campaign_ids TEXT NOT NULL, -- JSON array of campaign IDs
    campaign_set_id BIGINT NULL, -- For campaign set reports
    format VARCHAR(10) NOT NULL, -- 'word' or 'excel'
    status VARCHAR(20) NOT NULL DEFAULT 'queued', -- 'queued', 'processing', 'completed', 'failed'
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    started_at DATETIME NULL,
    completed_at DATETIME NULL,
    file_path VARCHAR(500) NULL,
    file_name VARCHAR(255) NULL,
    file_size BIGINT NULL,
    options_json TEXT NULL, -- JSON of report options (GDPR settings, etc.)
    error_message TEXT NULL,
    expires_at DATETIME NULL,
    PRIMARY KEY (id),
    KEY idx_reports_user_id (user_id),
    KEY idx_reports_status (status),
    KEY idx_reports_created_at (created_at),
    KEY idx_reports_expires_at (expires_at),
    CONSTRAINT fk_reports_user_id FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back

DROP TABLE IF EXISTS reports;
