## Backend Plan

Status: Replace notes CRUD with Warp Panel API, job runner, and integrations.

1) Scaffolding
- Create folders: `cmd/server`, `internal/config`, `internal/db`, `internal/models`, `internal/repository`, `internal/service`, `internal/controller`, `internal/router`, `internal/middleware`, `internal/integrations`, `internal/jobs`.
- Add main entry in `cmd/server/main.go`.

2) Dependencies and Config
- Add Gin, GORM (postgres driver), pgx stdlib, viper/godotenv, testify.
- Add GitHub (`go-github` + `oauth2`), Cloudflare (`cloudflare-go`), Docker client or CLI wrapper.
- Implement config loader and `.env.example`.

3) Runtime Settings
- Persist settings in DB (base domain, GitHub token, Cloudflare token, cloudflared config path, single tunnel name).
- Read settings for workflows; fall back to env only for bootstrap defaults.
- Add config preview endpoint for cloudflared config.yml (read-only).

4) Auth and Sessions
- GitHub OAuth login with state validation.
- Allowlist users or org membership.
- Session cookies or JWT with short TTL.

5) Database Models
- User, Project, Deployment, Job, AuditLog.
- Auto-migrate on startup.
- Add onboarding state (per-user) to avoid repeating onboarding overlays.

6) Integrations
- GitHub: create repo from template, list repos, clone.
- Docker: check port usage, `docker compose up --build -d`, `docker compose ps`.
- Cloudflare: host-first DNS via `cloudflared tunnel route dns`; keep API-managed ingress optional.
- Cloudflared: local config preview + validation; update config.yml and restart host service safely.
  - Tunnel status: surface active tunnel and ingress entries.
  - Optional Docker tunnel path only for fully containerized hosts.

7) Host Worker Handoff
- Issue one-time host tokens for deploy jobs.
- Provide host endpoints to fetch job payload and stream logs back.
- Store token TTL + revocation in DB.
- Track job state transitions: `pending_host` -> `running` -> `success|failed`.

8) Job Runner
- Queue jobs with type + input.
- Run tasks asynchronously with log streaming.
- Record status and errors in DB.
- Support a "waiting for host" state for host-executed jobs.

9) API Endpoints
- Host worker: `/api/v1/host/jobs/:token`, `/api/v1/host/jobs/:token/logs`, `/api/v1/host/jobs/:token/complete`.
- Auth: `/auth/login`, `/auth/callback`, `/auth/me`, `/auth/logout`.
- Projects: list, create-from-template, deploy-existing, quick-service.
- Jobs: list, get status, get logs.
- Health: `/healthz`, `/health/docker`, `/health/tunnel`.
- Onboarding: `GET /api/v1/onboarding` and `PATCH /api/v1/onboarding` (store completion per user).

10) Validation and Safety
- Validate names, ports, and filesystem paths.
- Never accept raw shell commands from the UI.
- Use an allowlist for subdomain formats and template repo.
- Ensure host tokens are single-use with short TTL.

11) Observability
- Structured logs with request IDs.
- Audit log for every deploy action.
- Add streaming endpoint for live container logs (all running containers, filterable by name/ID).
- Capture host-worker log events in job log stream.

12) Dockerization
- Multi-stage Dockerfile for API.
- Compose mounts for docker socket, templates dir, and cloudflared config.
