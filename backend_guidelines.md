## Backend Guidelines (Go + Gin + GORM)

### Stack and Libraries
- Go 1.22+.
- Gin for HTTP APIs.
- GORM v2 with pgx driver for Postgres.
- GitHub: `github.com/google/go-github/v62` + `golang.org/x/oauth2`.
- Cloudflare: API-based DNS + tunnel config for remote-managed tunnels; `cloudflared` CLI for tunnel status and local config preview.
- Docker control: use Docker socket and `docker compose` CLI or Docker client (`github.com/docker/docker/client`).
- Config: Viper or env-only loader; use `.env` for local overrides.
- Testing: `net/http/httptest`, `github.com/stretchr/testify`.

### Architecture
- Layers: models -> repositories -> services -> controllers -> routes.
- Separate external integrations into `internal/integrations` (github, cloudflare, docker, cloudflared).
- Job runner executes long-running tasks with status + logs; avoid blocking HTTP handlers.
- Avoid arbitrary shell; only allow whitelisted actions with validated inputs.

### Configuration (env)
- `APP_ENV=local|prod`
- `PORT=8080`
- `DATABASE_URL=postgres://user:pass@host:5432/warp?sslmode=disable`
- `TEMPLATES_DIR=/templates`
- `CLOUDFLARED_CONFIG=~/.cloudflared/config.yml`
- `CLOUDFLARED_TUNNEL_NAME=sphynx-app`
- `CLOUDFLARE_TUNNEL_ID=...` (tunnel UUID fallback when name lookups fail)
- `CLOUDFLARED_CREDENTIALS=/home/user/.cloudflared/xxxx.json`
- `DOMAIN=sphynx.store`
- `COOKIE_DOMAIN=sphynx.store`
- `ADMIN_LOGIN=admin`
- `ADMIN_PASSWORD=secret`
- `GITHUB_CLIENT_ID=...`
- `GITHUB_CLIENT_SECRET=...`
- `GITHUB_CALLBACK_URL=https://panel.yourdomain/callback`
- `GITHUB_ALLOWED_USERS=user1,user2`
- `GITHUB_ALLOWED_ORG=your-org`
- `GITHUB_TOKEN=...`
- `GITHUB_TEMPLATE_OWNER=Hrafngud`
- `GITHUB_TEMPLATE_REPO=go-ground`
- `GITHUB_REPO_OWNER=your-org`
- `GITHUB_REPO_PRIVATE=true`
- `CLOUDFLARE_API_TOKEN=...` (optional fallback if UI token not set)
- `CLOUDFLARE_ACCOUNT_ID=...` (required for API-managed tunnel updates)
- `CLOUDFLARE_ZONE_ID=...` (required for API-managed tunnel updates)
- `CLOUDFLARE_ACCOUNT_ID=...` (optional fallback)
- `CLOUDFLARE_ZONE_ID=...` (optional fallback)
Note: UI-managed settings (domain, GitHub token, Cloudflare token, cloudflared config path) should override env defaults.

### Data Model (suggested)
- User: GitHubID (stored as `git_hub_id` by GORM), login, avatar, last_login_at.
- Project: name, repo_url, path, proxy_port, db_port, status.
- Deployment: project_id, subdomain, hostname, port, state, last_run_at.
- Job: type, status, started_at, finished_at, error, log_lines.
- AuditLog: user_id, action, target, metadata.
- OnboardingState: user_id, completed_at, dismissed_steps (JSON or bool flags).

### GitHub OAuth and API
- Use OAuth login for UI access. Require allowlist (user or org membership).
- Store GitHub access token encrypted at rest or in server session (short-lived).
- Use template repo creation endpoint instead of `gh` CLI.

### Cloudflare Integration
- Primary path: host-installed `cloudflared` service with local `~/.cloudflared/config.yml`.
- `deploy.sh` is reference-only; do not modify it. The UI should mirror its CLI behavior before advanced automation.
- Add DNS records via `cloudflared tunnel route dns <UUID|NAME> <hostname>` (requires `cert.pem`).
- Update local `config.yml` ingress rules (insert after `ingress:`) and always keep a catch-all rule (`http_status:404`).
- Restart the host service after updates (`systemctl restart cloudflared`); validate with `cloudflared tunnel ingress validate`.
- Prefer host-worker execution (`deploy.sh` worker) so the API container does not edit tunnel state directly.
- Keep API-managed remote tunnels optional and explicitly non-primary.

### Docker and Host Actions
- Prefer Docker socket for container control; fall back to `docker compose` CLI if needed.
- Bind mount `TEMPLATES_DIR`, cloudflared config, and credentials into the API container.
- Validate ports using net listener checks plus Docker port scans.
- Run long tasks via job queue; provide log streaming to the UI.

### HTTP API Patterns
- JSON only; request/response DTOs separate from DB models.
- Routes under `/api/v1/`.
- Health at `/healthz`; auth-protected routes for actions.
- Consistent error shape: `{"error":"message"}`.

### Testing
- Unit-test services with fakes for GitHub/Cloudflare/Docker clients.
- Integration tests for job flows with mock adapters and temp filesystem.

### Docker
- Multi-stage Dockerfile for API.
- Compose should mount:
  - `/var/run/docker.sock`
  - templates directory
  - `~/.cloudflared` or specific files
