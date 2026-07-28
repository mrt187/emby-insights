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

External service credentials are intentionally not part of this first container foundation. They are added together with the Emby login and connector integration.
