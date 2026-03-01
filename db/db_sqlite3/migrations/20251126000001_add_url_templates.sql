-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied
CREATE TABLE IF NOT EXISTS url_templates (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    name VARCHAR(255) NOT NULL,
    url TEXT NOT NULL,
    category VARCHAR(255) NOT NULL,
    is_preset BOOLEAN NOT NULL DEFAULT 0,
    created_at DATETIME,
    updated_at DATETIME
);

CREATE INDEX idx_url_templates_user_id ON url_templates(user_id);
CREATE INDEX idx_url_templates_is_preset ON url_templates(is_preset);
CREATE INDEX idx_url_templates_category ON url_templates(category);

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back
DROP TABLE url_templates;
