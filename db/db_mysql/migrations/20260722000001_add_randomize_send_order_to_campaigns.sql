-- +goose Up
ALTER TABLE campaigns ADD COLUMN randomize_send_order BOOLEAN NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE campaigns DROP COLUMN randomize_send_order;
