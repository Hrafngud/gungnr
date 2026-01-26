<div align="center">


# Gungnr

</div>

Gungnr is a dockerized control panel for deploying template-based projects and
exposing local services via Cloudflare Tunnel. It runs on Linux hosts with access to Docker, 
the templates directory, and the local cloudflared config, and serves a web UI behind 
an nginx proxy.

Why I did it:

The `deploy.sh` silly little shell script tells alot about the history of this project:
  Basically, it was a collection of automations to manage templates for projects so I could start coding with the whole HTTPS and deploy stuff right away. From my computer.
  But since I also intended to expand the features after using it for a while, I decided to create a more sophisticated full stack application, that would allow me to expand the capabilities even further.

This project is perfect for simple development and test environment, personal use, and even share a machine with friends or colleagues to run simple useful service.
I'm planning to expand the list of docker service presets avaliable in the future, as well as the tech stack templates.
  
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

## Compatibility
Currently, Gungnr is **only supported on Linux**. We are looking forward to introducing a compatibility layer for other operating systems soon.
- Linux (amd64/arm64) with `apt`, `dnf`, `yum`, `pacman`, `apk`, or `zypper` package managers.
- Installer requires Bash (`install.sh` is a bash script). Run it from Bash even if your login shell is zsh.
- macOS and Windows support is planned for future releases.

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

## Documentation
Live: https://docs.jdoss.pro
Local source: `docs/index.html` (landing), `docs/docs.html` (docs), `docs/errors.html` (errors).
(If you fork you can continue to document etc.)

## Roadmap

### Current Features
- **One-command installer & bootstrap**: `install.sh` verifies dependencies; `gungnr bootstrap` configures tunnel, DNS, and environment in one flow.
- **GitHub OAuth authentication**: Secure login with role-based access control (SuperUser, Admin, User).
- **Deploy flows**:
  - Create from template: Start fresh projects from curated fullstack templates (currently Vue + Go + PostgreSQL).
  - Deploy existing: Forward local applications (localhost:PORT) to subdomains.
  - Quick services: Pull Docker registry images and expose via tunnel for rapid testing.
- **Cloudflare integration**: Locally managed tunnels with ingress routing and DNS management via Cloudflare API.
- **Docker-based runtime**: Compose orchestration with container logs, job history, and audit trails in the UI.
- **Role-based access control (RBAC)**: SuperUsers manage everything; Admins have most privileges but can't assign roles; Users can deploy and run jobs but can't manage allowlist.
- **CLI operations**: `gungnr restart`, `gungnr tunnel run`, and `gungnr uninstall` commands for panel and tunnel control.

### Planned Features
- **Expanded RBAC**: More granular permissions and role customization.
- **Additional templates**: Support for different fullstack stacks beyond the current Vue + Go + PostgreSQL.
- **Enhanced bootstrap**: Idempotent re-runs and safe upgrade paths.
- **Daemon management**: Optional auto-restart for cloudflared with systemd integration.
- **Additional CLI commands**: More panel and tunnel control operations.
- **macOS support**: Compatibility layer for macOS (amd64/arm64) via native installer.
- **Windows support**: PowerShell-based installation and management flows.
- **Windows support**: PowerShell-based installation and management flows.

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
