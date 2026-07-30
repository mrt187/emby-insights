-- Setup wizard: the first Emby account to log in successfully becomes the
-- Emby Insights admin (admin_owner), and Seerr/Radarr/Sonarr/TMDB/library
-- configuration moves from .env into app_config, editable at runtime via the
-- Verwaltung admin UI instead of requiring a container restart.
--
-- Both tables are singletons (id fixed to 1 via CHECK) so Postgres itself
-- guarantees at most one row: concurrent first logins race on the same
-- INSERT ... ON CONFLICT DO NOTHING, and exactly one wins.
CREATE TABLE admin_owner (
    id smallint PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    emby_user_id TEXT NOT NULL,
    assigned_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE app_config (
    id smallint PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    emby_device_id TEXT NOT NULL DEFAULT '',
    new_for_you_library_ids TEXT[] NOT NULL DEFAULT '{}',
    watched_library_ids TEXT[] NOT NULL DEFAULT '{}',
    seerr_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    seerr_base_url TEXT NOT NULL DEFAULT '',
    seerr_api_key_encrypted TEXT NOT NULL DEFAULT '',
    radarr_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    radarr_base_url TEXT NOT NULL DEFAULT '',
    radarr_api_key_encrypted TEXT NOT NULL DEFAULT '',
    sonarr_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    sonarr_base_url TEXT NOT NULL DEFAULT '',
    sonarr_api_key_encrypted TEXT NOT NULL DEFAULT '',
    tmdb_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    tmdb_api_key_encrypted TEXT NOT NULL DEFAULT '',
    comingsoon_region TEXT NOT NULL DEFAULT 'DE',
    comingsoon_days_ahead INT NOT NULL DEFAULT 28,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
