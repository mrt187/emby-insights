#!/usr/bin/env bash
set -euo pipefail

APP_DIR="${APP_DIR:-/mnt/cache/appdata/emby-insights}"
REPO_DIR="${REPO_DIR:-$APP_DIR/build}"
COMPOSE_FILE="${COMPOSE_FILE:-$APP_DIR/docker-compose.yml}"
IMAGE="${IMAGE:-registry.example.com/homelab/emby-insights}"
STATE_FILE="$APP_DIR/.emby-insights-release"
LOCK_FILE="$APP_DIR/.emby-insights-release.lock"

exec 9>"$LOCK_FILE"
flock -n 9 || exit 0

[[ -d "$REPO_DIR/.git" ]] || { echo "Build repository is missing: $REPO_DIR" >&2; exit 1; }
[[ -f "$COMPOSE_FILE" ]] || { echo "Compose file is missing: $COMPOSE_FILE" >&2; exit 1; }

git -C "$REPO_DIR" fetch --quiet --tags origin

release_tag="$(git -C "$REPO_DIR" tag --points-at origin/main --list 'v[0-9]*' | sort -V | tail -n 1)"
[[ -n "$release_tag" ]] || exit 0

release_version="${release_tag#v}"
release_commit="$(git -C "$REPO_DIR" rev-parse "$release_tag^{commit}")"
release_short_commit="${release_commit:0:12}"

if [[ -f "$STATE_FILE" ]] && [[ "$(<"$STATE_FILE")" == "$release_tag $release_commit" ]]; then
  exit 0
fi

if [[ -n "$(git -C "$REPO_DIR" status --porcelain)" ]]; then
  echo "Build repository has uncommitted changes; refusing to replace it." >&2
  exit 1
fi

git -C "$REPO_DIR" checkout --quiet --detach "$release_tag"

if [[ "$(<"$REPO_DIR/VERSION")" != "$release_version" ]]; then
  echo "Release tag and VERSION file do not match." >&2
  exit 1
fi

docker build --pull \
  --file "$REPO_DIR/docker/all-in-one/Dockerfile" \
  --tag "$IMAGE:$release_version" \
  --tag "$IMAGE:$release_short_commit" \
  --tag "$IMAGE:latest" \
  "$REPO_DIR"

docker push "$IMAGE:$release_version"
docker push "$IMAGE:$release_short_commit"
docker push "$IMAGE:latest"

docker pull "$IMAGE:latest"

# The first deployment may have been started outside of Compose. Remove only
# this explicitly named, replaceable application container so Compose can take
# ownership and recreate it with the freshly published `latest` image.
if docker container inspect emby-insights >/dev/null 2>&1; then
  docker rm --force emby-insights
fi

docker compose --file "$COMPOSE_FILE" up --detach --pull always --no-deps emby-insights

printf '%s %s\n' "$release_tag" "$release_commit" > "$STATE_FILE"
echo "Emby Insights $release_tag deployed ($release_short_commit)."
