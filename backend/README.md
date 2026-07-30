# Emby Insights backend

The Go backend is the only component the browser may call. It authenticates Emby users and aggregates data from the Emby Insights connector, Radarr, Sonarr, TMDB and Seerr.

## Current endpoints

- `GET /healthz` confirms that the HTTP service is running.
- `GET /readyz` confirms that PostgreSQL and Redis are reachable.

## Local configuration

Copy `.env.example` to `.env` and provide real local connection values. The `.env` file is intentionally ignored by Git.

`.env` only needs to hold values required before the database is reachable
(Emby base URL, Postgres/Redis connection, the encryption key for stored
secrets). Everything else — Seerr/Radarr/Sonarr/TMDB, library selection, and
who the admin is — is set up through the Verwaltung admin UI: the first Emby
account to log in becomes the Emby Insights admin automatically, and that
account configures the rest from the browser instead of editing `.env`.

Use the project-local Go SDK:

```text
tools/go/bin/go test ./...
```
