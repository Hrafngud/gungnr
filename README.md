<div align="center">

<svg viewBox="124 82 252 338" width="120" height="160" xmlns="http://www.w3.org/2000/svg">
  <path d="M263.00,342.00 L235.00,343.00 L229.00,352.00 L248.00,417.00 L253.00,414.00 L268.00,361.00 L269.00,352.00 Z" fill="currentColor" fill-rule="evenodd"/>
  <path d="M175.00,308.00 L181.00,316.00 L196.00,324.00 L236.00,332.00 L264.00,332.00 L297.00,326.00 L318.00,316.00 L324.00,307.00 L320.00,298.00 L303.00,289.00 L276.00,284.00 L275.00,287.00 L296.00,295.00 L300.00,299.00 L300.00,305.00 L289.00,311.00 L273.00,315.00 L227.00,315.00 L200.00,306.00 L199.00,299.00 L208.00,292.00 L224.00,288.00 L224.00,284.00 L189.00,292.00 L177.00,300.00 Z" fill="currentColor" fill-rule="evenodd"/>
  <path d="M248.00,244.00 L242.00,248.00 L242.00,255.00 L246.00,259.00 L246.00,270.00 L236.00,279.00 L236.00,304.00 L262.00,305.00 L263.00,279.00 L253.00,270.00 L253.00,260.00 L257.00,256.00 L257.00,248.00 Z" fill="currentColor" fill-rule="evenodd"/>
  <path d="M277.00,139.00 L281.00,159.00 L312.00,175.00 L332.00,195.00 L347.00,221.00 L353.00,248.00 L352.00,277.00 L341.00,307.00 L331.00,322.00 L314.00,339.00 L303.00,347.00 L281.00,356.00 L276.00,376.00 L282.00,376.00 L318.00,360.00 L331.00,350.00 L350.00,329.00 L367.00,296.00 L371.00,280.00 L373.00,253.00 L371.00,236.00 L358.00,199.00 L342.00,177.00 L329.00,164.00 L310.00,151.00 L288.00,141.00 Z" fill="currentColor" fill-rule="evenodd"/>
  <path d="M217.00,139.00 L201.00,145.00 L175.00,160.00 L151.00,184.00 L138.00,205.00 L130.00,227.00 L126.00,266.00 L130.00,290.00 L138.00,311.00 L156.00,338.00 L186.00,363.00 L207.00,373.00 L222.00,376.00 L218.00,356.00 L189.00,342.00 L164.00,317.00 L152.00,295.00 L145.00,264.00 L149.00,230.00 L160.00,205.00 L173.00,188.00 L187.00,175.00 L199.00,167.00 L217.00,160.00 L222.00,139.00 Z" fill="currentColor" fill-rule="evenodd"/>
  <path d="M246.00,84.00 L216.00,217.00 L222.00,226.00 L222.00,231.00 L211.00,233.00 L208.00,249.00 L231.00,267.00 L237.00,263.00 L236.00,247.00 L240.00,242.00 L246.00,104.00 L247.00,96.00 L252.00,95.00 L258.00,240.00 L263.00,248.00 L261.00,262.00 L268.00,267.00 L287.00,253.00 L291.00,246.00 L287.00,231.00 L277.00,231.00 L277.00,226.00 L282.00,220.00 L279.00,199.00 L253.00,84.00 Z" fill="currentColor" fill-rule="evenodd"/>
</svg>

# Gungnr

</div>

Gungnr is a dockerized control panel for deploying template-based projects and
exposing local services via Cloudflare Tunnel. It runs on Linux hosts with access to Docker, 
the templates directory, and the local cloudflared config, and serves a web UI behind 
an nginx proxy.

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

### Tunnel auto-start watchdog
Bootstrap installs a lightweight cron watchdog so `cloudflared` restarts after
reboot and is re-checked every 5 minutes.

Scripts created under `~/gungnr/state`:
- `~/gungnr/state/cloudflared-run.sh` (starts the tunnel using the generated config)
- `~/gungnr/state/cloudflared-ensure.sh` (checks the process and relaunches if needed)

Run the ensure script manually:
```bash
~/gungnr/state/cloudflared-ensure.sh
```

View the managed crontab entries:
```bash
crontab -l | rg 'gungnr-cloudflared'
```

Remove only the watchdog entries (leaves other cron jobs intact):
```bash
crontab -l | rg -v 'gungnr-cloudflared' | crontab -
```

## Gungnr setup
1) Run `./install.sh` to install the CLI and prerequisites.
2) Run `gungnr bootstrap` and follow the prompts to configure the machine.
3) Open the printed panel URL and login via GitHub.
4) Configure GitHub App settings in the UI if you want to enable template creation.

## Documentation
External docs are published from `docs/` via GitHub Pages.
Live URL: https://hrafngud.github.io/gungnr/
Local source: `docs/index.html` (landing), `docs/docs.html` (docs), `docs/errors.html` (errors).

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

## Release compose (GHCR images)
Use `docker-compose.release.yml` to run GHCR images instead of local builds.

Default image tag is `latest` (set by `GUNGNR_VERSION`). Pin a specific release
by exporting `GUNGNR_VERSION=vX.Y.Z` or editing the compose file directly.

Example:
```bash
GUNGNR_VERSION=v1.2.3 docker compose -f docker-compose.release.yml up -d
```

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
