-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied
CREATE TABLE IF NOT EXISTS "sms_requests" (
    "id" integer primary key autoincrement,
    "user_id" integer,
    "sms_template_id" integer,
    "page_id" integer,
    "sms_id" integer,
    "url" varchar(255),
    "r_id" varchar(255),
    "first_name" varchar(255),
    "last_name" varchar(255),
    "email" varchar(255),
    "phone" varchar(255),
    "position" varchar(255),
    "custom" varchar(255)
);

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back
DROP TABLE "sms_requests";
