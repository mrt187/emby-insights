-- Lets a user dismiss a series from "Noch nicht fertig" without having to
-- actually finish watching it. Scoped to that one row (not a generic hide
-- flag) since media_tracking already carries other independent per-title
-- state (rating, watchlist) that this must not interact with.
ALTER TABLE media_tracking ADD COLUMN hidden_in_progress BOOLEAN NOT NULL DEFAULT FALSE;
