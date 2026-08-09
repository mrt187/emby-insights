# All-in-one container

Backend, PostgreSQL and Redis in a single container with one persistent
`/config` volume. Runs on any Linux host with Docker.

The [main README](../../README.md) has the `docker-compose.yml` and `.env` to
copy, plus what each optional service adds. This page covers the details you
only need once something is unusual: what the variables mean, and how the
container behaves at runtime.

## Start

```bash
# Create docker-compose.yml and .env from the main README, then:
docker compose up -d
```

## Variables

| Variable | Meaning |
| --- | --- |
| `EMBY_INSIGHTS_IMAGE` | Required, no default. Registry image or your own build tag. |
| `EMBY_INSIGHTS_PORT` | Host port, defaults to `8081`. |
| `LISTEN_ADDRESS` | Only change this if port 8080 is already taken inside the container. |
| `POSTGRES_PASSWORD` | Pick your own long value. Do not change it after the first start — the existing database would become unusable. |
| `APP_ENCRYPTION_KEY` | Generate once with `openssl rand -base64 32` and keep it stable. Encrypts the stored API keys of Seerr, Radarr, Sonarr, TMDB, OMDb and Tracearr; losing it makes them unreadable. |
| `EMBY_BASE_URL` | Address of the Emby server, including the `/emby` path. |
| `EMBY_ADMIN_API_KEY` | Create it in Emby under Dashboard → Advanced → Security → API Keys. |
| `COOKIE_SECURE` | Keep `true` when reaching Emby Insights over HTTPS (e.g. behind a reverse proxy). Set to `false` only for plain HTTP without TLS — otherwise the browser silently drops the session cookie and the UI looks logged out or shows no data. |
| `TRUSTED_PROXIES` | Optional, comma-separated IPs or CIDRs of the reverse proxy in front. Only these sources may report the real client address via `X-Forwarded-For`. Leave unset when the container is reached directly. |
| `VAPID_PUBLIC_KEY` / `VAPID_PRIVATE_KEY` | Required since v0.13.0 for web push. Generate once with `npx web-push generate-vapid-keys` and keep stable — rotating them invalidates every existing browser subscription. |
| `VAPID_SUBJECT` | Required since v0.13.0. Your contact address for push services, as `mailto:you@example.com`. |
| `PUSH_POLL_INTERVAL` | Optional. How often the background poller checks for new content to notify about. Any Go duration, e.g. `15m`, `30m`, `1h`. Defaults to `20m`. |

Everything else — which services are enabled, their addresses and keys, and the
library selection — is configured in the admin UI after the first login. The
first Emby account to log in successfully becomes the administrator.

## Operation

Inside the container the API listens on port `8080`:

- `GET /healthz` — the process is running
- `GET /readyz` — PostgreSQL and Redis are reachable

Emby passwords are never stored. After login only the temporary Emby access
token lives in Redis. The Emby device ID is generated on first start and kept
in PostgreSQL.

Database migrations run automatically at startup, guarded by an advisory lock,
so overlapping containers during an Unraid "Update Stack" never apply the same
migration twice.
