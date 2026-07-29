-- user_settings/notifications reference user_mappings, a table no code has
-- ever populated — every insert into either table would fail on the FK.
ALTER TABLE user_settings DROP CONSTRAINT IF EXISTS user_settings_emby_user_id_fkey;
ALTER TABLE notifications DROP CONSTRAINT IF EXISTS notifications_emby_user_id_fkey;

CREATE TABLE media_tracking (
    emby_user_id TEXT NOT NULL,
    media_source TEXT NOT NULL,
    media_id TEXT NOT NULL,
    media_type TEXT NOT NULL,
    title TEXT NOT NULL,
    poster_url TEXT,
    rating SMALLINT CHECK (rating BETWEEN 1 AND 5),
    on_watchlist BOOLEAN NOT NULL DEFAULT FALSE,
    watched_on DATE,
    rewatch_count INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (emby_user_id, media_source, media_id)
);

CREATE INDEX media_tracking_watchlist_idx ON media_tracking (emby_user_id) WHERE on_watchlist;
