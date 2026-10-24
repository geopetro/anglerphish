-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied

-- Holds a single encrypted check value, used to verify that a supplied
-- encryption key matches the key the data was encrypted with. The key itself
-- is never stored.
CREATE TABLE IF NOT EXISTS encryption_meta (
  id BIGINT(20) NOT NULL AUTO_INCREMENT,
  check_value TEXT NOT NULL,
  created_at DATETIME NOT NULL,
  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back
DROP TABLE IF EXISTS encryption_meta;
