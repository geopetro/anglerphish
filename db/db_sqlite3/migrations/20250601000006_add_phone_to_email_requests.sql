-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied
ALTER TABLE email_requests ADD COLUMN phone VARCHAR(255);

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back
-- SQLite doesn't support dropping columns directly, so we need to recreate the table
CREATE TABLE email_requests_temp (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER,
    template_id INTEGER,
    page_id INTEGER,
    first_name VARCHAR(255),
    last_name VARCHAR(255),
    email VARCHAR(255),
    position VARCHAR(255),
    url VARCHAR(255),
    r_id VARCHAR(255),
    from_address VARCHAR(255),
    custom VARCHAR(255)
);

INSERT INTO email_requests_temp SELECT id, user_id, template_id, page_id, first_name, last_name, email, position, url, r_id, from_address, custom FROM email_requests;

DROP TABLE email_requests;

ALTER TABLE email_requests_temp RENAME TO email_requests;
