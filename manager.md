## Warp Panel - Project Tracker

- Scope: Web control panel to perform all deploy.sh tasks from a browser. It runs on the host PC (Docker, filesystem, cloudflared access) and is served via Cloudflare Tunnel for remote control.
- Core flows:
  - Create new project from a GitHub template, clone locally, patch compose ports, build and run.
  - Deploy an existing local template project (compose up if needed).
  - Expose a quick local service (any running port) via tunnel + DNS.
  - Manage tunnel ingress and DNS for subdomains on the configured zone.
- Deliverables:
  - Go API with authenticated GitHub login, Cloudflare and GitHub integrations, job runner, audit logs.
  - Vue UI for project wizard, status dashboards, and activity logs.
  - Docker Compose stack with Postgres, API, and web frontend.
- Tech decisions:
  - Backend: Go 1.22+, Gin, GORM + pgx, go-github, cloudflare-go, oauth2.
  - Frontend: Vue 3 + Vite + TS, Tailwind CSS v4, Pinia, Axios, vue-router.
  - DB: Postgres for projects, jobs, and audit logs.
  - Host access: Docker socket bind, templates dir bind, cloudflared config bind.
- Progress log:
  - TODO: Replace placeholder notes docs with Warp Panel plans and guidelines.
  - TODO: Implement GitHub OAuth login + allowlist.
  - TODO: Implement job runner for template creation/deploy/quick-service flows.
  - TODO: Build UI shell and connect to API.

### Docker / Compose usage (target)
- Defaults live in `.env.example` (copy to `.env` to override).
- Build + run: `docker compose up --build`.
- Stop and clean volumes: `docker compose down -v`.
- Ports: API `http://localhost:8080/healthz`, frontend `http://localhost:4173`, Postgres `localhost:5432`.

### Risks to track
- Security: browser-accessible control panel must strictly restrict who can run host actions.
- Secrets: GitHub and Cloudflare tokens stored safely (env or encrypted at rest).
- Safety: job runner must avoid arbitrary command execution and sanitize inputs.
