# Warp Panel

Warp Panel is a dockerized control panel for deploying template-based projects and
exposing local services via Cloudflare Tunnel. It runs on the host machine with
access to Docker, the templates directory, and the local cloudflared config, and
serves a web UI behind an nginx proxy.

Important: Do not edit `deploy.sh`. It is reference-only; the UI must mirror its CLI behavior before advanced automation.

## Architecture
- Go API + Postgres + job runner.
- Vue 3 UI served by nginx.
- Single nginx proxy on port 80 routes /, /api, and /auth.
- `cloudflared` runs on the host as a system service (primary path).
- Deploy actions run inside the API container via the Docker socket (no host-worker flow).
- `deploy.sh` is reference-only; do not modify it. The UI must reproduce its CLI behavior before any advanced automation.

## Requirements
- Docker + Docker Compose v2.
- `cloudflared` installed on the host with config + credentials, running as a system service.
- GitHub OAuth app for login.
- Optional GitHub token for template creation.
- Optional Cloudflare API token for API-managed tunnels (non-primary path).

## Recommended tunnel setup (host-installed)
This project assumes a locally-managed tunnel with `cloudflared` running as a system
service on the host. This is the primary path; the panel updates DNS/ingress via
the Cloudflare API when configured, but tunnel setup and service management stay
on the host.

1) Install `cloudflared` on the host.
   - Follow the official install guide for your OS:
     https://developers.cloudflare.com/cloudflare-one/networks/connectors/cloudflare-tunnel/do-more-with-tunnels/local-management/create-local-tunnel/#1-download-and-install-cloudflared
2) Authenticate the host with Cloudflare:
   - `cloudflared tunnel login`
3) Create a named tunnel:
   - `cloudflared tunnel create <TUNNEL_NAME>`
4) Create `~/.cloudflared/config.yml` with ingress rules.
   - Example (note the required catch-all rule):
     ```yml
     tunnel: <TUNNEL_UUID>
     credentials-file: /home/<user>/.cloudflared/<TUNNEL_UUID>.json
     ingress:
       - hostname: app.example.com
         service: http://localhost:8080
       - service: http_status:404
     ```
5) Create DNS records for hostnames you will use:
   - `cloudflared tunnel route dns <UUID|NAME> app.example.com`
   - This requires `cert.pem` in the default `.cloudflared` directory.
6) Install and run `cloudflared` as a service:
   - `cloudflared service install`
   - `systemctl start cloudflared`
   - Restart after config changes: `systemctl restart cloudflared`
7) (Optional) Validate ingress rules before restarting:
   - `cloudflared tunnel ingress validate`
   - `cloudflared tunnel ingress rule https://app.example.com`

## Warp Panel setup
1) Copy `.env.example` to `.env` and fill in required values.
2) Ensure the templates and cloudflared directories exist on the host.
3) Start the stack: `make up`.
4) Open `http://localhost` (or the tunnel hostname) and login via GitHub.
5) When you deploy, the API executes Docker commands over the host socket and then
   applies Cloudflare DNS/ingress updates.

## Environment configuration
Required for login:
- `SESSION_SECRET`
- `GITHUB_CLIENT_ID`
- `GITHUB_CLIENT_SECRET`
- `GITHUB_CALLBACK_URL` (use `http://localhost/auth/callback` when using the
  proxy, or your public host)
Optional access control:
- `GITHUB_ALLOWED_USERS`, `GITHUB_ALLOWED_ORG`
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

Note: Settings in the UI (domain, GitHub token, Cloudflare token, cloudflared
tunnel ref, cloudflared config path) override env defaults.
Cloudflare tokens should be API tokens (not global API keys) with
Account:Cloudflare Tunnel:Edit and Zone:DNS:Edit for the configured account and
zone.

If you are managing ingress via the Cloudflare API, ensure the tunnel is
remote-managed (`config_src=cloudflare`) and `cloudflared` is running on the
host as a service. This is an optional, non-primary path. Locally managed
tunnels (`config_src=local`) cannot be updated via the Cloudflare API.

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
- Create from template: choose a name and subdomain; Warp Panel creates the repo
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
- OAuth callback mismatch: update `GITHUB_CALLBACK_URL` to match the public host.
- cloudflared config missing: confirm `CLOUDFLARED_DIR` and
  `CLOUDFLARED_CONFIG`.
- Validate ingress rules: `cloudflared tunnel ingress validate`
- Test rule matching: `cloudflared tunnel ingress rule https://sub.example.com`
