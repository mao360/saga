#!/usr/bin/env bash
# Deploy the whole stack on the VPS from prebuilt GHCR images.
# Run from the repo root. Requires the env files to be configured for the VPS
# (ADMIN_BIND=127.0.0.1, strong passwords, etc. — see docs/deploy.md) and the
# host to be able to pull ghcr.io/mao360/saga-* (public packages or `docker login`).
set -euo pipefail

: "${IMAGE_TAG:?set IMAGE_TAG (git SHA or 'latest')}"
export IMAGE_TAG

cd "$(dirname "$0")/.."

echo "==> infra + monitoring"
docker compose --env-file .env.infra -f docker-compose.infra.yaml up -d
docker compose --env-file .env.monitoring -f docker-compose.monitoring.yaml up -d

for svc in order inventory payment; do
  echo "==> $svc (IMAGE_TAG=$IMAGE_TAG)"
  docker compose -f "$svc/docker-compose.yaml" pull
  docker compose -f "$svc/docker-compose.yaml" up -d
done

docker image prune -f >/dev/null 2>&1 || true
echo "==> deployed IMAGE_TAG=$IMAGE_TAG"