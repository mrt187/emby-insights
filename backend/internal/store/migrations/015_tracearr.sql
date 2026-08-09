-- Optional Tracearr integration. Tracearr does not replace the Emby Insights
-- connector plugin: watch time, devices, hours and weekdays keep coming from
-- Playback Reporting. This only adds the source for the insights Playback
-- Reporting never records — genres per play, unfinished plays, household
-- watchers and transcode share — and stays off until an operator enables it.
ALTER TABLE app_config
    ADD COLUMN tracearr_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN tracearr_base_url TEXT NOT NULL DEFAULT '',
    ADD COLUMN tracearr_api_key_encrypted TEXT NOT NULL DEFAULT '';
