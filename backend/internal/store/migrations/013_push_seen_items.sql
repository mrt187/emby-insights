-- Tracks which "New For You" items the background push poller has already
-- notified a user about, so re-running the poller doesn't re-send a push for
-- the same episode/movie every tick. item_id is the Emby item id.
CREATE TABLE push_seen_items (
    emby_user_id TEXT NOT NULL REFERENCES user_mappings (emby_user_id) ON DELETE CASCADE,
    item_id TEXT NOT NULL,
    seen_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (emby_user_id, item_id)
);
