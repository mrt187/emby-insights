-- The rewatch counter was a manual "I watched this again" tap with no
-- feature ever reading it back (no stats view, no list) — just a confusing
-- button with no visible effect. Removed rather than kept unused.
ALTER TABLE media_tracking DROP COLUMN IF EXISTS rewatch_count;
