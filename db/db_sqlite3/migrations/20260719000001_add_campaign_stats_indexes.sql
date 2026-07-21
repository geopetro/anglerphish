-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied

-- getCampaignStats filters both tables by campaign_id on every stats call and
-- sorts events by time. Without these, both are full table scans plus a temp
-- B-tree sort. Harmless on small datasets; significant at production scale.
CREATE INDEX IF NOT EXISTS idx_results_campaign_id ON results(campaign_id);
CREATE INDEX IF NOT EXISTS idx_events_campaign_id_time ON events(campaign_id, time);

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back
DROP INDEX IF EXISTS idx_results_campaign_id;
DROP INDEX IF EXISTS idx_events_campaign_id_time;
