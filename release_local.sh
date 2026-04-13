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

write_checksums() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$@"
    return
  fi

  if command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$@"
    return
  fi

  echo "Error: missing required command: sha256sum or shasum" >&2
  exit 1
}

VERSION="${1:-${VERSION:-}}"
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CLI_BUILD_SCRIPT="${ROOT_DIR}/scripts/build_gungnr.sh"

if [ -z "${VERSION}" ]; then
  usage
  exit 1
fi

require_cmd git
require_cmd go

mkdir -p dist

for GOOS in linux darwin; do
  for GOARCH in amd64 arm64; do
    CGO_ENABLED=0 GOOS=$GOOS GOARCH=$GOARCH \
      GUNGNR_VERSION="$VERSION" \
      EXTRA_LDFLAGS="-s -w" \
      OUTPUT="${ROOT_DIR}/dist/gungnr_${GOOS}_${GOARCH}" \
      "$CLI_BUILD_SCRIPT"
  done
done

write_checksums dist/gungnr_linux_* dist/gungnr_darwin_* > dist/checksums.txt

echo "Checksums written to dist/checksums.txt"

echo "Done. Native CLI builds are available in dist/ for:"
echo "  linux/amd64"
echo "  linux/arm64"
echo "  darwin/amd64"
echo "  darwin/arm64"
echo "Checksums:"
echo "  dist/checksums.txt"
