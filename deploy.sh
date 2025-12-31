#!/bin/bash

# deploy.sh - Final version: fully automated tunnel config updates
# No more manual edits needed!

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TEMPLATES_DIR="$ROOT_DIR/templates"
CONFIG_FILE="$HOME/.cloudflared/config.yml"
CREDENTIALS_FILE="$HOME/.cloudflared/b099639e-8f20-4365-9329-288816d20e14.json"
TUNNEL_NAME="sphynx-app"
DOMAIN="sphynx.store"
GITHUB_TEMPLATE="Hrafngud/go-ground"

# Prerequisites
for cmd in docker docker-compose cloudflared gh; do
  command -v "$cmd" >/dev/null || { echo "$cmd not found"; exit 1; }
done

[[ -f "$CREDENTIALS_FILE" ]] || { echo "Credentials missing: $CREDENTIALS_FILE"; exit 1; }
[[ -f "$CONFIG_FILE" ]] || { echo "Config missing: $CONFIG_FILE"; exit 1; }

# Helper: find genuinely free host port
find_free_port() {
  local base=$1
  local port=$base
  while true; do
    if lsof -iTCP:"$port" -sTCP:LISTEN >/dev/null 2>&1; then
      ((port++)); continue
    fi
    if docker ps --format '{{.Ports}}' | grep -q "0.0.0.0:$port->\|[::]:$port->"; then
      ((port++)); continue
    fi
    echo "$port"
    return
  done
}

# Helper: update config.yml - robust version with better validation
update_tunnel_config() {
  local hostname=$1
  local port=$2

  cp "$CONFIG_FILE" "${CONFIG_FILE}.bak"

  # Check if the hostname already exists (avoid duplicates)
  if grep -q "hostname: $hostname" "$CONFIG_FILE"; then
    echo "Warning: $hostname already exists in config - skipping insert"
    return
  fi

  # Insert new rule right after the ingress: line
  awk -v new_rule="  - hostname: $hostname\n    service: http://localhost:$port" \
      -v inserted=0 \
  '
  /^ingress:/ && !inserted {
    print $0
    print new_rule
    inserted=1
    next
  }
  { print }
  ' "$CONFIG_FILE" > "${CONFIG_FILE}.tmp" && mv -f "${CONFIG_FILE}.tmp" "$CONFIG_FILE"

  # Verify catch-all still exists
  if ! grep -q "service: http_status:404" "$CONFIG_FILE"; then
    echo "ERROR: Catch-all rule missing after update!"
    mv "${CONFIG_FILE}.bak" "$CONFIG_FILE"
    exit 1
  fi

  echo "Tunnel config updated: $hostname → localhost:$port"
}

# Helper: restart tunnel
restart_tunnel() {
  echo "Restarting Cloudflare tunnel..."
  pkill -f "cloudflared tunnel run.*$TUNNEL_NAME" || true
  nohup cloudflared tunnel run "$TUNNEL_NAME" >/dev/null 2>&1 &
}

# ==================== MODE: warp template ====================
if [[ "${1:-}" == "template" ]]; then
  echo "Creating new project from GitHub template: $GITHUB_TEMPLATE"

  read -p "Enter new project name: " PROJECT_NAME
  PROJECT_NAME="${PROJECT_NAME,,}"
  [[ -z "$PROJECT_NAME" ]] && { echo "Name required"; exit 1; }

  PROJECT_DIR="$TEMPLATES_DIR/$PROJECT_NAME"
  [[ -e "$PROJECT_DIR" ]] && { echo "Already exists: $PROJECT_DIR"; exit 1; }

  cd "$TEMPLATES_DIR"
  gh repo create "$PROJECT_NAME" --template "$GITHUB_TEMPLATE" --private --clone

  echo "Finding free host ports..."
  FREE_PROXY_PORT=$(find_free_port 80)
  FREE_DB_PORT=$(find_free_port 5432)

  echo "Using proxy port: $FREE_PROXY_PORT | DB port: $FREE_DB_PORT"

  COMPOSE_FILE="$PROJECT_DIR/docker-compose.yml"
  sed -i "s|- \"80:80\"|- \"${FREE_PROXY_PORT}:80\"|" "$COMPOSE_FILE"
  sed -i "s|\${DB_PORT:-5432}:5432|${FREE_DB_PORT}:5432|" "$COMPOSE_FILE"

  echo "Patched docker-compose.yml"

  read -p "Subdomain [default: $PROJECT_NAME]: " SUBDOMAIN
  SUBDOMAIN="${SUBDOMAIN:-$PROJECT_NAME}"
  SUBDOMAIN="${SUBDOMAIN,,}"
  HOSTNAME="$SUBDOMAIN.$DOMAIN"

  echo "Starting containers..."
  cd "$PROJECT_DIR"
  docker compose up --build -d

  echo "Adding DNS route..."
  cloudflared tunnel route dns "$TUNNEL_NAME" "$HOSTNAME"

  update_tunnel_config "$HOSTNAME" "$FREE_PROXY_PORT"

  restart_tunnel

  echo ""
  echo "=== DEPLOYED ==="
  echo "https://$HOSTNAME"
  echo "DB: psql -h localhost -p $FREE_DB_PORT -U notes notes"
  echo "Stop: cd $PROJECT_DIR && docker compose down"
  exit 0
fi

# ==================== MODE: warp (existing/quick) ====================
echo "Deploy existing template or quick local service"

mapfile -t PROJECTS < <(find "$TEMPLATES_DIR" -mindepth 1 -maxdepth 1 -type d -not -name '.*' | xargs -n1 basename | sort)
OPTIONS=("${PROJECTS[@]}" "Quick local service")

select OPT in "${OPTIONS[@]}"; do
  [[ "$OPT" == "Quick local service" ]] && QUICK_MODE=true || { QUICK_MODE=false; PROJECT="$OPT"; PROJECT_DIR="$TEMPLATES_DIR/$PROJECT"; }
  break
done

read -p "Subdomain: " SUBDOMAIN
SUBDOMAIN="${SUBDOMAIN,,}"
[[ -z "$SUBDOMAIN" ]] && { echo "Required"; exit 1; }
HOSTNAME="$SUBDOMAIN.$DOMAIN"

if [[ "$QUICK_MODE" == true ]]; then
  read -p "Local port (service already running): " PORT
  [[ "$PORT" =~ ^[0-9]+$ && "$PORT" -ge 1 && "$PORT" -le 65535 ]] || { echo "Invalid port"; exit 1; }
else
  [[ -f "$PROJECT_DIR/docker-compose.yml" ]] || { echo "No compose file"; exit 1; }
  read -p "Host port to expose [default 80]: " PORT
  PORT="${PORT:-80}"
  echo "Starting $PROJECT..."
  cd "$PROJECT_DIR"
  docker compose up --build -d
fi

cloudflared tunnel route dns "$TUNNEL_NAME" "$HOSTNAME"
update_tunnel_config "$HOSTNAME" "$PORT"
restart_tunnel

echo ""
echo "=== LIVE ==="
echo "https://$HOSTNAME"
[[ "$QUICK_MODE" == false ]] && echo "Stop: cd $PROJECT_DIR && docker compose down"
echo "Tunnel restarted automatically"
