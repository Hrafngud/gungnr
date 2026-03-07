#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    printf 'Error: missing required command: %s\n' "$1" >&2
    exit 1
  fi
}

git_value() {
  local args=("$@")
  if ! command -v git >/dev/null 2>&1; then
    return 0
  fi

  git -C "$ROOT_DIR" "${args[@]}" 2>/dev/null || true
}

default_version() {
  local tag
  tag="$(git_value describe --tags --exact-match HEAD)"
  if [ -n "$tag" ]; then
    printf '%s\n' "$tag"
    return
  fi

  printf 'dev\n'
}

default_commit() {
  local commit
  commit="$(git_value rev-parse HEAD)"
  if [ -n "$commit" ]; then
    printf '%s\n' "$commit"
    return
  fi

  printf 'none\n'
}

default_date() {
  local commit_date
  commit_date="$(git_value log -1 --format=%cI HEAD)"
  if [ -n "$commit_date" ]; then
    printf '%s\n' "$commit_date"
    return
  fi

  printf 'unknown\n'
}

require_cmd go

TARGET_OS="${GOOS:-$(go env GOOS)}"
TARGET_ARCH="${GOARCH:-$(go env GOARCH)}"
OUTPUT="${OUTPUT:-$ROOT_DIR/dist/gungnr_${TARGET_OS}_${TARGET_ARCH}}"
VERSION_VALUE="${GUNGNR_VERSION:-${VERSION:-$(default_version)}}"
COMMIT_VALUE="${GUNGNR_COMMIT:-${COMMIT:-$(default_commit)}}"
DATE_VALUE="${GUNGNR_BUILD_DATE:-${DATE:-$(default_date)}}"
PACKAGE="${PACKAGE:-./cmd/gungnr}"

mkdir -p "$(dirname "$OUTPUT")"

LDFLAGS="-X main.version=${VERSION_VALUE} -X main.commit=${COMMIT_VALUE} -X main.date=${DATE_VALUE}"

if [ -n "${EXTRA_LDFLAGS:-}" ]; then
  LDFLAGS="${LDFLAGS} ${EXTRA_LDFLAGS}"
fi

(
  cd "$ROOT_DIR"
  go build -trimpath -ldflags "$LDFLAGS" -o "$OUTPUT" "$PACKAGE"
)

printf 'Built %s\n' "$OUTPUT"
printf 'Version metadata: version=%s commit=%s date=%s\n' "$VERSION_VALUE" "$COMMIT_VALUE" "$DATE_VALUE"
