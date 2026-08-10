-- Moves the VAPID web push keypair from an operator-supplied .env value into
-- the same runtime-configurable app_config row as Seerr/Radarr/Sonarr/TMDB,
-- so enabling push no longer needs a container restart or an Unraid
-- template field (which Unraid's Community Applications caches locally and
-- never refetches for an already-installed container, see README history).
-- The private key is encrypted like the other stored API keys; the public
-- key is not a secret and is served to the browser as-is.
ALTER TABLE app_config
    ADD COLUMN push_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN push_subject TEXT NOT NULL DEFAULT '',
    ADD COLUMN push_public_key TEXT NOT NULL DEFAULT '',
    ADD COLUMN push_private_key_encrypted TEXT NOT NULL DEFAULT '';
