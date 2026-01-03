## Backend Plan

Status: Replace notes CRUD with Warp Panel API, job runner, and integrations (API-run Docker runner in place).

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
- Docker: API-runner via socket for `docker run` (quick service) and `docker compose up` (templates).
- Docker: check port usage, handle container name collisions, expose container logs.
- Docker: add lifecycle controls (stop/restart/remove) with explicit stop-vs-remove semantics (remove can optionally delete volumes).
- Cloudflare: host-first DNS via `cloudflared tunnel route dns`; keep API-managed ingress optional.
- Cloudflared: local config preview + validation; update config.yml and restart host service safely.
  - Tunnel status: surface active tunnel and ingress entries.
  - Optional Docker tunnel path only for fully containerized hosts.

7) Docker Runner Jobs
- Add job types: `docker_run`, `docker_compose_up`.
- Quick service: run container first, then update tunnel ingress to the selected port.
- Template deploy: `docker compose up --build -d`, then update ingress to proxy port.
- Infer container name from image (e.g., `excalidraw/excalidraw` -> `excalidraw`).
- Ensure container naming rules when collisions occur (suffix with incrementing numbers).
- Stop keeps the container configuration intact for later restart; remove deletes the container, with an option to also remove volumes.
- API now invokes Docker via the host socket; host-worker flow is removed.

8) Job Runner
- Queue jobs with type + input.
- Run tasks asynchronously with log streaming.
- Record status and errors in DB.
- Support docker runner job lifecycle with clear logs.

9) API Endpoints
- Auth: `/auth/login`, `/auth/callback`, `/auth/me`, `/auth/logout`.
- Projects: list, create-from-template, deploy-existing, quick-service.
- Jobs: list, get status, get logs.
- Containers: list, stop, restart, remove (with optional volume deletion), logs.
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

12) Dockerization
- Multi-stage Dockerfile for API.
- Compose mounts for docker socket, templates dir, and cloudflared config.
