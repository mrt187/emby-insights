-- Tracks which items the background push poller has already notified a user
-- about, so re-running the poller doesn't re-send a push for the same item
-- every tick. item_id is a prefixed synthetic key ("newforyou:<id>",
-- "request:<id>", ...), not a bare Emby item id — see push_poller.go.
CREATE TABLE push_seen_items (
    emby_user_id TEXT NOT NULL REFERENCES user_mappings (emby_user_id) ON DELETE CASCADE,
    item_id TEXT NOT NULL,
    seen_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (emby_user_id, item_id)
);
