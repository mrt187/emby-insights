# Build the API as a static Linux binary. Runs on the builder's native
# platform (no QEMU emulation, which crashes the Go toolchain) and
# cross-compiles to the target via GOARCH below.
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS backend-build

WORKDIR /src/backend
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags='-s -w' -o /out/emby-insights ./cmd/server

FROM node:22-alpine AS frontend-build

WORKDIR /src/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

# PostgreSQL is the persistent store. Redis, the API and the frontend are supervised by the entrypoint.
FROM postgres:17-alpine

RUN apk add --no-cache redis nodejs

COPY --from=backend-build /out/emby-insights /usr/local/bin/emby-insights
COPY --from=frontend-build /src/frontend/dist /app/frontend/dist
COPY --from=frontend-build /src/frontend/node_modules /app/frontend/node_modules
COPY --from=frontend-build /src/frontend/package.json /app/frontend/package.json
COPY docker/all-in-one/entrypoint.sh /usr/local/bin/emby-insights-entrypoint

RUN chmod 755 /usr/local/bin/emby-insights-entrypoint

ENV PGDATA=/config/postgresql \
    REDIS_DIR=/config/redis \
    POSTGRES_DB=emby_insights \
    POSTGRES_USER=emby_insights \
    LISTEN_ADDRESS=:8080

VOLUME ["/config"]
EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/emby-insights-entrypoint"]
