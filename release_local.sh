#!/usr/bin/env bash
set -euo pipefail

usage() {
  echo "Usage: $0 vX.Y.Z"
  echo "Example: $0 v1.0.0"
}

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "Error: missing required command: $1" >&2
    exit 1
  fi
}

VERSION="${1:-${VERSION:-}}"
OWNER="${OWNER:-hrafngud}"
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CLI_BUILD_SCRIPT="${ROOT_DIR}/scripts/build_gungnr.sh"

if [ -z "${VERSION}" ]; then
  usage
  exit 1
fi

require_cmd git
require_cmd go
require_cmd docker
require_cmd sha256sum

if ! docker buildx version >/dev/null 2>&1; then
  echo "Error: docker buildx is required." >&2
  exit 1
fi

mkdir -p dist

for GOARCH in amd64 arm64; do
  GOOS=linux
  CGO_ENABLED=0 GOOS=$GOOS GOARCH=$GOARCH \
    GUNGNR_VERSION="$VERSION" \
    EXTRA_LDFLAGS="-s -w" \
    OUTPUT="${ROOT_DIR}/dist/gungnr_${GOOS}_${GOARCH}" \
    "$CLI_BUILD_SCRIPT"
done

sha256sum dist/gungnr_linux_* > dist/checksums.txt

echo "Checksums written to dist/checksums.txt"

if ! docker buildx inspect >/dev/null 2>&1; then
  docker buildx create --use
fi

API_IMAGE="ghcr.io/${OWNER}/gungnr-api"
WEB_IMAGE="ghcr.io/${OWNER}/gungnr-web"

echo "Pushing ${API_IMAGE}:${VERSION} and :latest"
docker buildx build --platform linux/amd64,linux/arm64 \
  -t "${API_IMAGE}:${VERSION}" \
  -t "${API_IMAGE}:latest" \
  -f backend/Dockerfile backend --push

echo "Pushing ${WEB_IMAGE}:${VERSION} and :latest"
docker buildx build --platform linux/amd64,linux/arm64 \
  --build-arg VITE_API_BASE_URL=/ \
  -t "${WEB_IMAGE}:${VERSION}" \
  -t "${WEB_IMAGE}:latest" \
  -f frontend/go-notes/Dockerfile frontend/go-notes --push

echo "Done. Upload these assets to the GitHub release ${VERSION}:"
echo "  dist/gungnr_linux_amd64"
echo "  dist/gungnr_linux_arm64"
echo "  dist/checksums.txt"
