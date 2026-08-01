# Unraid deployment

Copy this `deploy/unraid` directory to `/mnt/cache/appdata/emby-insights/`. It contains the Compose definition.

Before starting it in Compose Manager:

1. Copy `.env.example` to `.env`.
2. Set a unique `POSTGRES_PASSWORD`.
3. Add an Emby **administrator API key** as `EMBY_ADMIN_API_KEY`.
4. Generate an `APP_ENCRYPTION_KEY` with `openssl rand -base64 32`.
5. Keep `COOKIE_SECURE=false` only for the first direct HTTP test at port `8081`; set it to `true` before exposing the app through HTTPS.
6. Optionally set `EMBY_INSIGHTS_IMAGE` if you're pulling from your own registry instead of the default `ghcr.io/mrt187/emby-insights:latest`. If your setup needs a custom Docker network (e.g. a VPN container network), add it under `networks:` in `docker-compose.yml`.

The stack creates persistent application data in `./config`.
