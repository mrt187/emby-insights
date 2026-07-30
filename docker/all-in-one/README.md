# All-in-one container

This image contains the Emby Insights Go backend, PostgreSQL and Redis. It is intended for one Unraid container with a single persistent `/config` volume.

## Build

```text
docker build -f docker/all-in-one/Dockerfile -t emby-insights:local .
```

## Required configuration

Set `POSTGRES_PASSWORD` to a long, unique value before the first start. PostgreSQL data and Redis persistence are stored below `/config`.

The API listens on port `8080` and provides:

- `GET /healthz` — process is running
- `GET /readyz` — PostgreSQL and Redis are reachable

Set `EMBY_BASE_URL` (including `/emby`), `EMBY_ADMIN_API_KEY`, and `APP_ENCRYPTION_KEY` (generate with `openssl rand -base64 32`) as well. Emby passwords are never stored; after a successful login only the temporary Emby access token is retained in Redis for the dashboard session.

The Emby device id is generated automatically on first start and persisted in Postgres — no manual `uuidgen` step needed. Radarr, Sonarr, TMDB, Seerr and library selection are configured after deployment through the Verwaltung admin UI: the first Emby account to log in becomes the Emby Insights admin automatically.
