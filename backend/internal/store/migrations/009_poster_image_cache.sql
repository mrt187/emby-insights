-- Top Bewertet stored only a poster_url pointing at Emby's image proxy with
-- the image tag frozen at rating time. Once Emby regenerates that item's
-- artwork (library rescan, metadata refresh), the tag goes stale and the
-- poster 404s forever. Cache the actual bytes instead, so a rating's poster
-- survives Emby changing or even removing the title later.
ALTER TABLE media_tracking ADD COLUMN poster_image BYTEA;
ALTER TABLE media_tracking ADD COLUMN poster_content_type TEXT;
