-- The frontend was German-only, with every label hardcoded. The UI language
-- is now a global, admin-controlled setting alongside the other Verwaltung
-- values. It deliberately does not touch comingsoon_region: which regional
-- release dates and which metadata language Radarr/Sonarr/TMDB deliver stays
-- a separate decision from which language the interface chrome speaks.
ALTER TABLE app_config ADD COLUMN language TEXT NOT NULL DEFAULT 'de';
