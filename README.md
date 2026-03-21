<div align="center">

![Gungnr Logo](frontend/go-notes/public/logo.svg)

# Gungnr

</div>

Gungnr is a dockerized control panel for deploying template-based projects and
exposing local services via Cloudflare Tunnel. It runs on Linux hosts with access to Docker, 
the templates directory, and the local cloudflared config, and serves a web UI behind 
an nginx proxy.

Why I did it:

The `deploy.sh` silly little shell script tells a lot about the history of this project:
  Basically, it was a collection of automations to manage templates for projects so I could start coding with the whole HTTPS and deploy stuff right away. From my computer.
  But since I also intended to expand the features after using it for a while, I decided to create a more sophisticated full stack application, that would allow me to expand the capabilities even further.

This project is perfect for simple development and test environment, personal use, and even share a machine with friends or colleagues to run simple useful service.
I'm planning to expand the list of docker service presets avaliable in the future, as well as the tech stack templates.
  
## Architecture
- Go API + Postgres + job runner.
- Vue 3 UI served by nginx.
- Single nginx proxy on port 80 routes /, /api, and /auth.
- `cloudflared` runs on the host as a user-managed CLI process (primary path).
- API is the orchestration/control plane: it submits bridge intents and maps worker results to jobs/logs.
- Host lifecycle execution is handled by the host worker through the filesystem bridge queue.
- `deploy.sh` is reference-only; do not modify it. The UI must reproduce its CLI behavior before any advanced automation.

## Host lifecycle bridge (current state)
- Bridge cutover is complete: host lifecycle endpoints and async project restart run through bridge intents/results.
- API responsibilities: validate request + RBAC, submit task intents, wait/load results, map outcomes to API/job logs.
- Worker responsibilities: execute host lifecycle actions (`restart_tunnel`, container lifecycle, compose restart, host probes) and write structured task results.
- Queue storage defaults to `INFRA_QUEUE_ROOT=/templates/.infra` with `intents/`, `claims/`, `results/`, and `logs/` subdirectories.

## Requirements
- Cloudflared, Docker and Docker Compose on host machine.
- GitHub account.
- Cloudflare account (free tier) and a domain registered on Cloudflare.

## Compatibility
Currently, Gungnr is **only supported on Linux**. We are looking forward to introducing a compatibility layer for other operating systems soon.
- Linux (amd64/arm64) with `apt`, `dnf`, `yum`, `pacman`, `apk`, or `zypper` package managers.
- Installer requires Bash (`install.sh` is a bash script). Run it from Bash even if your login shell is zsh.
- macOS and Windows support is planned for future releases.

## Bootstrap-managed tunnel setup
The bootstrap CLI configures and runs a locally managed tunnel with `cloudflared`
as a user-managed process. Manual tunnel setup is no longer required for a
standard install.
Reboot fallback is available via keepalive tooling and bootstrap-installed watchdog scripts.

### Tunnel auto-start watchdog
`gungnr keepalive` now manages reboot persistence with a supervisor chain:
- preferred: system-level systemd units (`/etc/systemd/system/gungnr.service` + `/etc/systemd/system/gungnr-keepalive.timer`)
- fallback: user-level systemd units (`~/.config/systemd/user/gungnr-cloudflared-keepalive.service` + `.timer`)
- fallback: crontab watchdog (`@reboot` + every 5 minutes)

Scripts created under `~/gungnr/state`:
- `~/gungnr/state/cloudflared-run.sh` (starts the tunnel using the generated config)
- `~/gungnr/state/cloudflared-ensure.sh` (checks the process and relaunches if needed)

Common keepalive commands:
- `gungnr keepalive enable` (core mode: panel stack + tunnel)
- `gungnr keepalive all` (core + managed project recovery)
- `gungnr keepalive status`
- `gungnr keepalive disable`


## Installation

### Method 1 - Install Script (Recommended)

**0)** Do all Cloudflare and Github prerequisite config [as instructed on docs](https://docs.jdoss.pro/docs.html#install-steps)!!

**1)** Run:
  ```
   wget -qO /tmp/gungnr-install.sh https://raw.githubusercontent.com/Hrafngud/gungnr/main/install.sh && chmd +x /tmp/gungnr-install.sh && /tmp/gungnr-install.sh
  ```
   to install the CLI and prerequisites.

**2)** Run `gungnr bootstrap` and follow the prompts to configure the application.

**3)** Open the printed panel URL and login via GitHub.

**4)** Configure GitHub App settings in the UI if you want to enable template creation (optional).


---

### Method 2 - Build From Source (For Development)

**0)** Do all Cloudflare and Github prerequisite config [as instructed on docs](https://docs.jdoss.pro/docs.html#install-steps)!!

**1)** Create a directory:
  ```
  mkdir gungnr-dev
  cd gungnr-dev
  ```
  then clone Gungnr:

  ```
  git clone https://github.com/Hrafngud/gungnr.git
  ```

  <em>OBS: If you clone it at `~/` , it will conflict with gungnr default config directory, do it into a separate directory.</em>
  
**2)** Build:
  ```
  go build -o ./gungnr ./cmd/gungnr

  ```
  
**3)** Run bootstrap:

  ```
  ./gungnr bootstrap
  ```
**4)** Open the printed panel URL and login via GitHub.

**5)** Configure GitHub App settings in the UI if you want to enable template creation (optional).

## Documentation
Live: https://docs.jdoss.pro

Local source: `docs/index.html` (landing), `docs/docs.html` (docs), `docs/errors.html` (errors).
(If you forked the repo, you can continue to document new features there.)

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
- **Daemon management for keepalive**: `gungnr keepalive` supports systemd integration (system-level first, with user-systemd/cron fallback) for cloudflared + stack recovery after reboot.
- **Role-based access control (RBAC)**: SuperUsers manage everything; Admins have most privileges but can't assign roles; Users can deploy and run jobs but can't manage allowlist.
- **CLI operations**: `gungnr restart`, `gungnr tunnel run`, `gungnr keepalive <enable|disable|status|all|recover>`, and `gungnr uninstall` commands for panel and tunnel control.

### Planned Features
- **Expanded RBAC**: Define clearer RBAC rules and presets for different case scenarios.
- **Additional templates**: Support for different fullstack stacks beyond the current Vue + Go + PostgreSQL.
- **Enhanced bootstrap**: Idempotent re-runs and safe upgrade paths.
- **Additional CLI commands**: More panel and tunnel control operations.
- **Interoperability**: Modern support for integration with external tools (API's, MCP's etc.).
- **macOS support**: Compatibility layer for macOS (amd64/arm64) via native installer.
- **Windows support**: PowerShell-based installation and management flows.
- **Feedback and support official channels**: Currently, if you have an problem you can open an issue directly.
- **Console and Filesystem**: Adding console and filesystem views. 
- **TUI installer persistence**: Persisting the installation state for a while even when user shutdown the TUI.
- **Advanced Docker deployment**: For a broad compatibility with images that require custom setup before startup.
- **More one click deployments**: Testing and validating more tools for quick deployment.

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

## Test token auth (optional)

Since only OAuth is supported, when you wish to hit the panel with curl, or give a token to an agent/test suite,
you can leverage this endpoint for grabbing a token, this is optional and requires setting up the variables in .env:

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
- `make build-cli`
- `make logs`
- `make down`
- `make down-v`

## CLI build metadata
Build the CLI through the shared helper so `gungnr version` reports consistent
ldflags-backed provenance:

```bash
make build-cli
./dist/gungnr_$(go env GOOS)_$(go env GOARCH) version
```

The shared contract is:
- `GUNGNR_VERSION`: defaults to the exact tag at `HEAD`, otherwise `dev`
- `GUNGNR_COMMIT`: defaults to `git rev-parse HEAD`
- `GUNGNR_BUILD_DATE`: defaults to `git log -1 --format=%cI HEAD`

`release_local.sh` reuses the same contract for release artifacts and only
overrides `GUNGNR_VERSION` explicitly.

Pushing a SemVer tag like `v1.2.3` triggers
`.github/workflows/release-cli.yml`, which rebuilds `gungnr` for
`linux/amd64`, `linux/arm64`, `darwin/amd64`, and `darwin/arm64` through the
same helper, uploads those binaries plus `checksums.txt` to the matching
GitHub Release, and publishes the multi-arch container images
`ghcr.io/hrafngud/gungnr-api` and `ghcr.io/hrafngud/gungnr-web` with tags
`vX.Y.Z` and `latest`.

## Release compose (GHCR images)
Use `docker-compose.release.yml` to run GHCR images instead of local builds.

Published image names:
- `ghcr.io/hrafngud/gungnr-api`
- `ghcr.io/hrafngud/gungnr-web`

Default image tag is `latest` (set by `GUNGNR_VERSION`). Pin a specific release
by exporting `GUNGNR_VERSION=vX.Y.Z` or editing the compose file directly.

Example:
```bash
GUNGNR_VERSION=v1.2.3 docker compose -f docker-compose.release.yml up -d
```

## Workflows
- Create from template: choose a name and subdomain; Gungnr creates the repo
  and deploys it.
- Forward localhost service: provide a service name, subdomain, and local host
  port to expose an existing app through the tunnel.
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
