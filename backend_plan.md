## Backend Plan

Status: Replace notes CRUD with Warp Panel API, job runner, and integrations.

1) Scaffolding
- Create folders: `cmd/server`, `internal/config`, `internal/db`, `internal/models`, `internal/repository`, `internal/service`, `internal/controller`, `internal/router`, `internal/middleware`, `internal/integrations`, `internal/jobs`.
- Add main entry in `cmd/server/main.go`.

2) Dependencies and Config
- Add Gin, GORM (postgres driver), pgx stdlib, viper/godotenv, testify.
- Add GitHub (`go-github` + `oauth2`), Cloudflare (`cloudflare-go`), Docker client or CLI wrapper.
- Implement config loader and `.env.example`.

3) Auth and Sessions
- GitHub OAuth login with state validation.
- Allowlist users or org membership.
- Session cookies or JWT with short TTL.

4) Database Models
- User, Project, Deployment, Job, AuditLog.
- Auto-migrate on startup.

5) Integrations
- GitHub: create repo from template, list repos, clone.
- Docker: check port usage, `docker compose up --build -d`, `docker compose ps`.
- Cloudflare: add DNS record and update tunnel ingress.
- Cloudflared: update config.yml and restart tunnel safely.

6) Job Runner
- Queue jobs with type + input.
- Run tasks asynchronously with log streaming.
- Record status and errors in DB.

7) API Endpoints
- Auth: `/auth/login`, `/auth/callback`, `/auth/me`, `/auth/logout`.
- Projects: list, create-from-template, deploy-existing, quick-service.
- Jobs: list, get status, get logs.
- Health: `/healthz`, `/health/docker`, `/health/tunnel`.

8) Validation and Safety
- Validate names, ports, and filesystem paths.
- Never accept raw shell commands from the UI.
- Use an allowlist for subdomain formats and template repo.

9) Observability
- Structured logs with request IDs.
- Audit log for every deploy action.

10) Dockerization
- Multi-stage Dockerfile for API.
- Compose mounts for docker socket, templates dir, and cloudflared config.
