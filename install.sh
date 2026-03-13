#!/usr/bin/env bash
set -euo pipefail

log() {
  printf '%s\n' "$*"
}

log_stderr() {
  printf '%s\n' "$*" >&2
}

warn() {
  printf 'Warning: %s\n' "$*" >&2
}

die() {
  printf 'Error: %s\n' "$*" >&2
  exit 1
}

require_command() {
  command -v "$1" >/dev/null 2>&1
}

ensure_sudo() {
  if [ "$(id -u)" -eq 0 ]; then
    echo ""
    return
  fi

  if require_command sudo; then
    echo "sudo"
    return
  fi

  die "sudo is required to install system dependencies."
}

detect_os_arch() {
  local uname_os uname_arch
  uname_os="$(uname -s)"
  uname_arch="$(uname -m)"

  case "$uname_os" in
    Linux) OS="linux" ;;
    Darwin) OS="darwin" ;;
    *) die "Unsupported OS: $uname_os" ;;
  esac

  case "$uname_arch" in
    x86_64|amd64) ARCH="amd64" ;;
    arm64|aarch64) ARCH="arm64" ;;
    *) die "Unsupported architecture: $uname_arch" ;;
  esac
}

detect_pkg_manager() {
  if require_command apt-get; then
    PKG_MANAGER="apt"
  elif require_command dnf; then
    PKG_MANAGER="dnf"
  elif require_command yum; then
    PKG_MANAGER="yum"
  elif require_command pacman; then
    PKG_MANAGER="pacman"
  elif require_command apk; then
    PKG_MANAGER="apk"
  elif require_command zypper; then
    PKG_MANAGER="zypper"
  elif require_command brew; then
    PKG_MANAGER="brew"
  else
    PKG_MANAGER=""
  fi
}

install_packages() {
  local sudo_cmd
  sudo_cmd="$(ensure_sudo)"

  case "$PKG_MANAGER" in
    apt)
      $sudo_cmd apt-get update -y
      $sudo_cmd apt-get install -y "$@"
      ;;
    dnf)
      $sudo_cmd dnf install -y "$@"
      ;;
    yum)
      $sudo_cmd yum install -y "$@"
      ;;
    pacman)
      $sudo_cmd pacman -Sy --noconfirm "$@"
      ;;
    apk)
      $sudo_cmd apk add --no-cache "$@"
      ;;
    zypper)
      $sudo_cmd zypper --non-interactive install "$@"
      ;;
    brew)
      brew install "$@"
      ;;
    *)
      die "No supported package manager found. Install dependencies manually."
      ;;
  esac
}

ensure_docker() {
  if require_command docker; then
    return
  fi

  log "Docker not found. Installing..."
  detect_pkg_manager

  case "$OS:$PKG_MANAGER" in
    darwin:brew)
      brew install --cask docker
      ;;
    linux:apt)
      install_packages docker.io
      ;;
    linux:dnf)
      install_packages docker
      ;;
    linux:yum)
      install_packages docker
      ;;
    linux:pacman)
      install_packages docker
      ;;
    linux:apk)
      install_packages docker
      ;;
    linux:zypper)
      install_packages docker
      ;;
    *)
      die "Unable to install Docker automatically. Install Docker and retry."
      ;;
  esac

  if ! require_command docker; then
    die "Docker installation did not provide the docker command. Install Docker and retry."
  fi
}

has_docker_compose() {
  docker compose version >/dev/null 2>&1
}

ensure_docker_compose() {
  if has_docker_compose; then
    return
  fi

  log "Docker Compose not found. Installing..."
  detect_pkg_manager

  case "$OS:$PKG_MANAGER" in
    darwin:brew)
      if ! require_command docker; then
        die "Docker must be installed before Docker Compose. Install Docker and retry."
      fi
      ;;
    linux:apt)
      install_packages docker-compose
      ;;
    linux:dnf)
      install_packages docker-compose-plugin
      ;;
    linux:yum)
      install_packages docker-compose-plugin
      ;;
    linux:pacman)
      install_packages docker-compose
      ;;
    linux:apk)
      install_packages docker-cli-compose
      ;;
    linux:zypper)
      install_packages docker-compose
      ;;
    *)
      die "Unable to install Docker Compose automatically. Install Docker Compose v2 and retry."
      ;;
  esac

  if ! has_docker_compose; then
    die "Docker Compose is still unavailable. Install Docker Compose v2 and retry."
  fi
}

ensure_cloudflared() {
  if require_command cloudflared; then
    return
  fi

  log "cloudflared not found. Installing..."
  detect_pkg_manager

  case "$OS:$PKG_MANAGER" in
    darwin:brew)
      brew install cloudflared
      ;;
    linux:apt)
      install_packages cloudflared
      ;;
    linux:dnf)
      install_packages cloudflared
      ;;
    linux:yum)
      install_packages cloudflared
      ;;
    linux:pacman)
      install_packages cloudflared
      ;;
    linux:apk)
      install_packages cloudflared
      ;;
    linux:zypper)
      install_packages cloudflared
      ;;
    *)
      die "Unable to install cloudflared automatically. Install cloudflared and retry."
      ;;
  esac

  if ! require_command cloudflared; then
    die "cloudflared is still unavailable. Install cloudflared and retry."
  fi
}

detect_release_repo() {
  if [ -n "${GUNGNR_CLI_REPO:-}" ]; then
    return
  fi

  if require_command git && git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
    local remote_url repo_path
    remote_url="$(git config --get remote.origin.url || true)"
    repo_path=""
    if [[ "$remote_url" =~ github.com[:/](.+)/(.+)(\.git)?$ ]]; then
      repo_path="${BASH_REMATCH[1]}/${BASH_REMATCH[2]}"
    fi

    if [ -n "$repo_path" ]; then
      GUNGNR_CLI_REPO="${repo_path%.git}"
      return
    fi
  fi

  GUNGNR_CLI_REPO="hrafngud/gungnr"
  log_stderr "Using default release repo ${GUNGNR_CLI_REPO} (set GUNGNR_CLI_REPO to override)."
}

download_cli() {
  local tmp_dir url version asset_name release_base
  tmp_dir="$(mktemp -d)"
  version="${GUNGNR_CLI_VERSION:-latest}"
  asset_name="${GUNGNR_CLI_ASSET:-gungnr_${OS}_${ARCH}}"

  if [ -n "${GUNGNR_CLI_URL:-}" ]; then
    url="$GUNGNR_CLI_URL"
  else
    detect_release_repo
    if [ -z "$GUNGNR_CLI_REPO" ]; then
      die "Set GUNGNR_CLI_URL to the CLI download URL."
    fi

    if [ "$version" = "latest" ]; then
      release_base="https://github.com/${GUNGNR_CLI_REPO}/releases/latest/download"
    else
      release_base="https://github.com/${GUNGNR_CLI_REPO}/releases/download/${version}"
    fi

    url="${release_base}/${asset_name}"
  fi

  log_stderr "Downloading Gungnr CLI from ${url}"
  if require_command curl; then
    curl -fsSL "$url" -o "${tmp_dir}/gungnr"
  elif require_command wget; then
    wget -qO "${tmp_dir}/gungnr" "$url"
  else
    die "curl or wget is required to download the CLI."
  fi

  echo "$tmp_dir"
}

install_cli() {
  detect_os_arch

  local tmp_dir sudo_cmd
  tmp_dir="$(download_cli)"
  sudo_cmd="$(ensure_sudo)"

  $sudo_cmd install -m 0755 "${tmp_dir}/gungnr" /usr/local/bin/gungnr
  rm -rf "$tmp_dir"

  if ! require_command gungnr; then
    die "Failed to install gungnr to /usr/local/bin/gungnr."
  fi
}

main() {
  detect_os_arch
  ensure_docker
  ensure_docker_compose
  ensure_cloudflared
  install_cli

  log 'Run "gungnr bootstrap" to configure this machine.'
}

main "$@"
