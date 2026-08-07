-- New installations should default to English instead of German. Existing
-- rows are left untouched: an admin who already has language='de' keeps it,
-- this only changes what a fresh app_config row is seeded with.
ALTER TABLE app_config ALTER COLUMN language SET DEFAULT 'en';
