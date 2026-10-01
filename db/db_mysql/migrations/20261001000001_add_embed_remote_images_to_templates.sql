-- +goose Up
ALTER TABLE templates ADD COLUMN embed_remote_images BOOLEAN NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE templates DROP COLUMN embed_remote_images;
