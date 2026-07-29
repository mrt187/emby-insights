# Emby Insights backend

The Go backend is the only component the browser may call. It authenticates Emby users and aggregates data from the Emby Insights connector, Radarr, Sonarr, TMDB and Seerr.

## Current endpoints

- `GET /healthz` confirms that the HTTP service is running.
- `GET /readyz` confirms that PostgreSQL and Redis are reachable.

## Local configuration

Copy `.env.example` to `.env` and provide real local connection values. The `.env` file is intentionally ignored by Git.

Use the project-local Go SDK:

```text
tools/go/bin/go test ./...
```
