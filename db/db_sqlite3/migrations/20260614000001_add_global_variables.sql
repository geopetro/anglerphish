-- +goose Up
CREATE TABLE IF NOT EXISTS global_variables (
    user_id    INTEGER PRIMARY KEY,
    first_name TEXT NOT NULL DEFAULT '',
    last_name  TEXT NOT NULL DEFAULT '',
    email      TEXT NOT NULL DEFAULT '',
    phone      TEXT NOT NULL DEFAULT '',
    position   TEXT NOT NULL DEFAULT '',
    custom     TEXT NOT NULL DEFAULT ''
);

-- +goose Down
DROP TABLE IF EXISTS global_variables;
