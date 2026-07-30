-- Backs the Verwaltung activity chart: how many Seerr requests Emby
-- Insights actually triggered, and how many distinct users were active,
-- per day. seerr_request_log gets one row per successful request creation.
-- daily_activity gets at most one row per user per day (the app dedupes
-- writes via Redis before inserting) so "active users" is a plain COUNT
-- DISTINCT, not a decayed/estimated figure.
CREATE TABLE seerr_request_log (
    id BIGSERIAL PRIMARY KEY,
    emby_user_id TEXT NOT NULL,
    media_type TEXT NOT NULL,
    tmdb_id INTEGER NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX seerr_request_log_created_at_idx ON seerr_request_log (created_at);

CREATE TABLE daily_activity (
    emby_user_id TEXT NOT NULL,
    activity_date DATE NOT NULL,
    PRIMARY KEY (emby_user_id, activity_date)
);
CREATE INDEX daily_activity_date_idx ON daily_activity (activity_date);
