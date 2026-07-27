#!/usr/bin/env bash
#
# Build the local gateway images. These are NOT on a registry for local dev, so
# docker-compose.yml references plain tags (rurdesk-gateway-*:latest) that must
# exist in the local Docker daemon — this script builds them. Mirrors the build
# contexts used by CI (.github/workflows/docker-release.yml).
#
# Usage:  ./script/build-gateway.sh            # build both (goose + claude)
#         ./script/build-gateway.sh goose      # build only the goose gateway
#         ./script/build-gateway.sh claude     # build only the claude gateway
#
# The dev docker-compose.yml uses the GOOSE image; claude is built for parity
# with CI / in case you switch a bot's gateway.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

target="${1:-all}"

build_goose() {
  echo "==> Building rurdesk-gateway-goose:latest"
  docker build -t rurdesk-gateway-goose:latest -f gateway/goose/Dockerfile gateway
}

build_claude() {
  echo "==> Building rurdesk-gateway-claude:latest"
  docker build -t rurdesk-gateway-claude:latest -f gateway/claude-code/Dockerfile gateway
}

case "$target" in
  goose)  build_goose ;;
  claude) build_claude ;;
  all)    build_goose; build_claude ;;
  *) echo "unknown target: $target (use: goose | claude | all)" >&2; exit 2 ;;
esac

echo "==> Done. docker-compose.yml references rurdesk-gateway-goose:latest."
