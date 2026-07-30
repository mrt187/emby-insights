-- OMDb is optional, off by default: enriches the media-detail screen with
-- IMDb and Rotten Tomatoes ratings, which TMDB's own API does not expose
-- (TMDB only has its own community score). Same shape as tmdb_* — no base
-- URL, OMDb's endpoint is fixed.
ALTER TABLE app_config ADD COLUMN omdb_enabled BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE app_config ADD COLUMN omdb_api_key_encrypted TEXT NOT NULL DEFAULT '';
