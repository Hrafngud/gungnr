# Warp Panel

Warp Panel is a dockerized control panel for deploying template-based projects and
exposing local services via Cloudflare Tunnel. It runs on the host machine with
access to Docker, the templates directory, and the local cloudflared config, and
serves a web UI behind an nginx proxy.

## Architecture
- Go API + Postgres + job runner.
- Vue 3 UI served by nginx.
- Single nginx proxy on port 80 routes /, /api, and /auth.

## Requirements
- Docker + Docker Compose v2.
- cloudflared config and credentials on the host.
- GitHub OAuth app for login.
- Optional GitHub token for template creation.
- Optional Cloudflare API token for DNS automation.

## Setup
1) Copy `.env.example` to `.env` and fill in required values.
2) Ensure the templates and cloudflared directories exist on the host.
3) Start the stack: `make up`.
4) Open `http://localhost` (or the tunnel hostname) and login via GitHub.

## Environment configuration
Required for login:
- `SESSION_SECRET`
- `GITHUB_CLIENT_ID`
- `GITHUB_CLIENT_SECRET`
- `GITHUB_CALLBACK_URL` (use `http://localhost/auth/callback` when using the
  proxy, or your public host)
Optional access control:
- `GITHUB_ALLOWED_USERS`, `GITHUB_ALLOWED_ORG`

Host integration defaults:
- `TEMPLATES_DIR` (where template repos are cloned)
- `CLOUDFLARED_DIR` (directory with cloudflared config and credentials)
- `CLOUDFLARED_CONFIG` (path to config.yml inside the container, mounted from
  host)
- `DOMAIN`, `CLOUDFLARED_TUNNEL_NAME`, `CLOUDFLARE_API_TOKEN`

Note: Settings in the UI (domain, GitHub token, Cloudflare token, cloudflared
config path) override env defaults.

## Common commands
- `make up` (foreground)
- `make up-d` (detached)
- `make logs`
- `make down`
- `make down-v`

## Workflows
- Create from template: choose a name and subdomain; Warp Panel creates the repo
  and deploys it.
- Deploy existing: select a local template project, set a subdomain, and start
  compose.
- Quick local service: provide a subdomain and a running local port.
- Activity: review the audit timeline of user actions in the Activity view.

## Local development (optional)
- Backend: `cd backend && go run ./cmd/server`
- Frontend: `cd frontend/go-notes && npm install && \
VITE_API_BASE_URL=http://localhost:8080 npm run dev`
- Ensure `CORS_ALLOWED_ORIGINS` includes the dev origin (e.g.
  `http://localhost:5173`).

## Ports
- The nginx proxy exposes port 80 by default.
- The API and web ports are not exposed unless you uncomment them in
  `docker-compose.yml` and set `API_PORT` / `WEB_PORT`.

## Troubleshooting
- Check service health: `make ps`, `make logs`
- OAuth callback mismatch: update `GITHUB_CALLBACK_URL` to match the public host.
- cloudflared config missing: confirm `CLOUDFLARED_DIR` and
  `CLOUDFLARED_CONFIG`.
