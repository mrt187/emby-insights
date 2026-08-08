-- push_subscriptions/push_seen_items reference user_mappings, a table no
-- code has ever populated (see 002_media_tracking.sql, which fixed the same
-- bug for user_settings/notifications) — every insert into either table
-- fails on the FK.
ALTER TABLE push_subscriptions DROP CONSTRAINT IF EXISTS push_subscriptions_emby_user_id_fkey;
ALTER TABLE push_seen_items DROP CONSTRAINT IF EXISTS push_seen_items_emby_user_id_fkey;
