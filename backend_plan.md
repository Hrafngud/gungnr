## Backend Plan

Status: Replace notes CRUD with Warp Panel API, job runner, and integrations (API-run Docker runner in place).

1) Scaffolding
- Create folders: `cmd/server`, `internal/config`, `internal/db`, `internal/models`, `internal/repository`, `internal/service`, `internal/controller`, `internal/router`, `internal/middleware`, `internal/integrations`, `internal/jobs`.
- Add main entry in `cmd/server/main.go`.
  - Keep parity with `deploy.sh` behaviors (template generation, compose patching, cloudflared updates, DNS routing), but implemented via APIs.

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

4.1) Phase 2 - RBAC Security Layer (Step-by-step)
- Add `role` to users table (enum or text), default `user`.
- Likely wipe existing `users` table before seeding to avoid legacy allowlist state.
- Remove `GITHUB_ALLOWED_USERS` / `GITHUB_ALLOWED_ORG` logic from OAuth flow.
- On OAuth login: only allow users present in DB (allowlist lives in users table).
- Seed SuperUser from env (`SUPERUSER_GH_NAME`, `SUPER_GH_ID`) on startup.
- Enforce SuperUser cap (max 2); remove excess and shutdown panel if exceeded.
- Add role to session payload and include in `/auth/me`.
- Add role-aware middleware helpers (RequireAdmin, RequireSuperUser, RequireUser).
- Protect user-management routes (Admin/SuperUser) and add ownership checks for User.

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
 - Parity notes vs `deploy.sh`:
   - Port selection: mimic `find_free_port` logic and avoid host/Docker conflicts.
   - Ingress updates: preserve catch-all rule and avoid duplicate hostnames.
   - Tunnel restart semantics: match the "restart to apply new ingress" flow.

6.1) GitHub Template Generation (GitHub App Token)
- Review GitHub docs for template generation using ref.tools (`POST /repos/{template_owner}/{template_repo}/generate`).
- Endpoint details (REST API):
  - Headers: `Accept: application/vnd.github+json`.
  - Path params: `template_owner` (required), `template_repo` (required, no `.git`).
  - Body: `name` (required), `owner` (optional org/user), `description` (optional),
    `include_all_branches` (bool, default false), `private` (bool, default false).
  - Response: `201 Created` with repo payload.
  - Template repo must be marked as a template and accessible to the app installation, otherwise GitHub returns 404.
- Confirm token type and issuance flow: GitHub App user access token or installation token (no PATs).
- GitHub App requirements (persist in DB, entered via UI):
  - App creation link: https://github.com/settings/apps/new
  - Permissions: Repository Administration (write), Contents (read), Metadata (read).
  - Installation must include the template repo and the target owner (org/user).
  - Store App ID, Client ID, Client Secret, Installation ID, and App Private Key in settings.
- Auto-resolve installation ID (avoid manual UI entry):
  - Use app JWT to fetch installation by org/user/repo or list all app installations.
  - Prefer `GET /orgs/{org}/installation` or `GET /repos/{owner}/{repo}/installation` when owner/repo is known.
  - Fall back to `GET /app/installations` and let the UI select the matching account.
- Token minting flow (server-side):
  - Create JWT with App ID + private key.
  - Exchange JWT for installation token (or user access token if creating repos in user space).
  - Log token scope/permissions metadata for debugging (no secrets).
- Access troubleshooting:
  - `GET /repos/{owner}/{repo}` can return 404 when the GitHub App installation does not include the template repo or the repo is private/invisible to the installation.
  - Confirm the installation ID matches the template owner (org/user) and that the app is installed with access to the template repo.
  - For template generation, docs confirm GitHub App installation tokens are supported and require Administration (write) + Contents (read) permissions.
- Support owner selection (user vs org) and visibility in request payload.
- Support `include_all_branches` when generating from template if needed.
- Implement generate endpoint call with request validation (owner, template repo, new repo name, visibility).
- Handle async repo creation readiness (polling/backoff before clone/deploy).
- Add GitHub error handling with response payload snippets for debugging.
 - Ensure generated repo name is safe and unique (lowercase, hyphenated, avoid collisions).

6.2) Template Catalog (Optional)
- Store template catalog entries (owner, repo, display name, visibility, default flag) as UI convenience only.
- Extend `/api/v1/github/catalog` to return the template list, default template, and target owner hints.
- Allow create-from-template to accept a template source from the catalog, but do not enforce allowlist behavior yet.
- Document catalog ownership rules (template source vs target owner) and expose in settings UI.
- Ensure audit logs capture selected template source and target owner/visibility.

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
- Jobs: support pagination parameters (page/limit or cursor) for list endpoint.
- Containers: list, stop, restart, remove (with optional volume deletion), logs.
- Containers: list running + stopped (include status), filter support.
- Containers: filter by project ID/name and include volumes/images scoped to project.
- Host repos: list local template projects for lifecycle actions (stop/restart).
- Templates: list available template repos (from GitHub config + API).
- Forward local service: create/update Cloudflare DNS + ingress for a given localhost port (no Docker).
- Health: `/healthz`, `/health/docker`, `/health/tunnel`.
- Onboarding: `GET /api/v1/onboarding` and `PATCH /api/v1/onboarding` (store completion per user).

10) Validation and Safety
- Validate names, ports, and filesystem paths.
- Never accept raw shell commands from the UI.
- Validate subdomain formats and template repo identifiers (owner/repo).
- Ensure host tokens are single-use with short TTL.
 - Validate local repo discovery root path; prevent path traversal.
 - Validate project-based filters via known labels and DB mappings only.

11) Observability
- Structured logs with request IDs.
- Audit log for every deploy action.
- Add streaming endpoint for live container logs (all running containers, filterable by name/ID).

12) Dockerization
- Multi-stage Dockerfile for API.
- Compose mounts for docker socket, templates dir, and cloudflared config.

13) Host Resource Insights
- Add Docker usage stats endpoint (disk usage, images/containers/volumes count).
- Add basic host resource snapshot if feasible (optional CPU/memory via Docker API).
 - Define data source (`docker system df` or Docker API) and include counts + total size.
 - Keep response lightweight; avoid expensive calls on every page load.
