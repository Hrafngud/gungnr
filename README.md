# Gungnr

Gungnr is a dockerized control panel for deploying template-based projects and
exposing local services via Cloudflare Tunnel. It runs on the host machine with
access to Docker, the templates directory, and the local cloudflared config, and
serves a web UI behind an nginx proxy.

Important: Do not edit `deploy.sh`. It is reference-only; the UI must mirror its CLI behavior before advanced automation.
Setup is now driven by a one-time terminal bootstrap (`install.sh` + `gungnr bootstrap`).

## Architecture
- Go API + Postgres + job runner.
- Vue 3 UI served by nginx.
- Single nginx proxy on port 80 routes /, /api, and /auth.
- `cloudflared` runs on the host as a user-managed CLI process (primary path).
- Deploy actions run inside the API container via the Docker socket (no host-worker flow).
- `deploy.sh` is reference-only; do not modify it. The UI must reproduce its CLI behavior before any advanced automation.

## Requirements
- Sudo access to install dependencies.
- GitHub OAuth app credentials (Client ID/Secret + callback URL) for bootstrap input.
- Cloudflare account, domain, and API token with tunnel + DNS edit permissions for bootstrap input.
- `install.sh` installs or verifies Docker, Docker Compose v2, and `cloudflared`.

## Bootstrap-managed tunnel setup
The bootstrap CLI configures and runs a locally managed tunnel with `cloudflared`
as a user-managed process. Manual tunnel setup is no longer required for a
standard install.
Persistence and auto-restart are out of scope for now.

## Gungnr setup
1) Run `./install.sh` to install the CLI and prerequisites.
2) Run `gungnr bootstrap` and follow the prompts to configure the machine.
3) Open the printed panel URL and login via GitHub.
4) Configure GitHub App settings in the UI if you want to enable template creation.

## Environment configuration
The bootstrap CLI generates a complete `.env`. Reference values:
Required for login:
- `SESSION_SECRET`
- `GITHUB_CLIENT_ID`
- `GITHUB_CLIENT_SECRET`
- `GITHUB_CALLBACK_URL`
Optional access control:
Manage access via the Users allowlist in the panel (SuperUser/Admin only).
Admin test token (optional):
- `ADMIN_LOGIN`, `ADMIN_PASSWORD` to enable `POST /test-token` for a bearer token.

Host integration defaults:
- `TEMPLATES_DIR` (where template repos are cloned)
- `CLOUDFLARED_DIR` (directory with cloudflared config and credentials)
- `CLOUDFLARED_CONFIG` (path to config.yml inside the container, mounted from
  host)
- `DOMAIN`, `CLOUDFLARED_TUNNEL_NAME` (name or UUID), `CLOUDFLARE_TUNNEL_ID` (ID fallback), `CLOUDFLARE_API_TOKEN`
- `CLOUDFLARE_ACCOUNT_ID`, `CLOUDFLARE_ZONE_ID` (required for API-managed tunnels)
- `VITE_API_BASE_URL=/` when building the web container so the UI uses same-origin HTTPS.

Note: Host Settings are for inspection and minor adjustments only. Settings in
the UI (domain, GitHub token, Cloudflare token, cloudflared tunnel ref,
cloudflared config path) override env defaults.
Cloudflare tokens should be API tokens (not global API keys) with
Account:Cloudflare Tunnel:Edit and Zone:DNS:Edit for the configured account and
zone.

If you are managing ingress via the Cloudflare API, ensure the tunnel is
remote-managed (`config_src=cloudflare`) and `cloudflared` is running on the
host as a user-managed process. This is an optional, non-primary path. Locally
managed tunnels (`config_src=local`) cannot be updated via the Cloudflare API.

## Test token auth (optional)
If `ADMIN_LOGIN` and `ADMIN_PASSWORD` are set, you can request a bearer token:
```bash
curl -sS http://localhost/test-token \
  -H 'Content-Type: application/json' \
  -d '{"login":"admin","password":"secret"}'
```
Use the returned token as `Authorization: Bearer <token>` for `/api/v1/*` routes.

## Common commands
- `make up` (foreground)
- `make up-d` (detached)
- `make logs`
- `make down`
- `make down-v`

## Workflows
- Create from template: choose a name and subdomain; Gungnr creates the repo
  and deploys it.
- Deploy existing: select a local template project, set a subdomain, and start
  compose.
- Quick local service: provide a subdomain and host port (defaults to running an
  Excalidraw container on port 80).
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
- Validate ingress rules: `cloudflared tunnel ingress validate`
- Test rule matching: `cloudflared tunnel ingress rule https://sub.example.com`
