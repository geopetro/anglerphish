-- +goose Up
CREATE TABLE IF NOT EXISTS global_variables (
    user_id    BIGINT PRIMARY KEY,
    first_name VARCHAR(255) NOT NULL DEFAULT '',
    last_name  VARCHAR(255) NOT NULL DEFAULT '',
    email      VARCHAR(255) NOT NULL DEFAULT '',
    phone      VARCHAR(255) NOT NULL DEFAULT '',
    position   VARCHAR(255) NOT NULL DEFAULT '',
    custom     TEXT NOT NULL DEFAULT ''
);

-- +goose Down
DROP TABLE IF EXISTS global_variables;
