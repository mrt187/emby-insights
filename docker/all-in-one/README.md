# All-in-one container

This image contains the Emby Insights Go backend, PostgreSQL and Redis in one container, with a single persistent `/config` volume. Runs on any Linux host with Docker.

## Run with Docker Compose (recommended)

```bash
cp .env.example .env
# edit .env: set EMBY_INSIGHTS_IMAGE, POSTGRES_PASSWORD, EMBY_BASE_URL,
#            EMBY_ADMIN_API_KEY, APP_ENCRYPTION_KEY
docker compose up -d
```

`EMBY_INSIGHTS_IMAGE` has no default and Compose refuses to start without
it. There is no public image yet, so build from source (see below) and
point the variable at your own tag.

Emby passwords are never stored; after a successful login only the temporary Emby access token is retained in Redis for the dashboard session. The Emby device id is generated automatically on first start and persisted in Postgres — no manual `uuidgen` step needed. Radarr, Sonarr, TMDB, Seerr and library selection are configured after deployment through the Verwaltung admin UI: the first Emby account to log in becomes the Emby Insights admin automatically.

The API listens on port `8080` inside the container and provides:

- `GET /healthz` — process is running
- `GET /readyz` — PostgreSQL and Redis are reachable

### Unraid

Copy this `docker/all-in-one` directory to `/mnt/cache/appdata/emby-insights/`, then start it from Compose Manager the same way as above.

## Building the image yourself

```bash
docker build -f docker/all-in-one/Dockerfile -t emby-insights:local .
```

Then point `EMBY_INSIGHTS_IMAGE` in `.env` at your own tag.
