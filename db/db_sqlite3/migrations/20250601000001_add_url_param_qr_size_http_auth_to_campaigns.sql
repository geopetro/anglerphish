-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied
ALTER TABLE "campaigns" ADD COLUMN "url_param" varchar(255);
ALTER TABLE "campaigns" ADD COLUMN "qr_size" varchar(255);
ALTER TABLE "campaigns" ADD COLUMN "http_auth" integer;

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back
PRAGMA foreign_keys=off;
PRAGMA foreign_keys=on;
