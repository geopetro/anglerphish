-- +goose Up
ALTER TABLE draft_campaigns ADD COLUMN randomize_send_order BOOLEAN NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE draft_campaigns DROP COLUMN randomize_send_order;
