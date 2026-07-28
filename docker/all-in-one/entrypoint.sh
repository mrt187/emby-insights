#!/bin/sh
set -eu

: "${POSTGRES_PASSWORD:?POSTGRES_PASSWORD must be configured}"

mkdir -p "$PGDATA" "$REDIS_DIR"

postgres_pid=""
redis_pid=""
api_pid=""

cleanup() {
    trap - EXIT INT TERM
    for pid in "$api_pid" "$redis_pid" "$postgres_pid"; do
        if [ -n "$pid" ]; then
            kill -TERM "$pid" 2>/dev/null || true
            wait "$pid" 2>/dev/null || true
        fi
    done
}

trap cleanup EXIT INT TERM

docker-entrypoint.sh postgres &
postgres_pid=$!

until pg_isready --host 127.0.0.1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" >/dev/null 2>&1; do
    sleep 1
done

redis-server --dir "$REDIS_DIR" --appendonly yes --save 60 1 &
redis_pid=$!

export DATABASE_URL="${DATABASE_URL:-postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@127.0.0.1:5432/${POSTGRES_DB}?sslmode=disable}"
export REDIS_URL="${REDIS_URL:-redis://127.0.0.1:6379/0}"

/usr/local/bin/emby-insights &
api_pid=$!

wait "$api_pid"
