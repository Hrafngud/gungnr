## Go Notes – Project Tracker

- Scope: Full-stack notes app with Go (Gin + GORM/pgx) backend, Vue 3 + TS + Vite frontend, PostgreSQL, Tailwind v4, Pinia, Axios, Docker Compose for one-command run.
- Deliverables: CRUD API with clean layering; responsive UI with note list/detail/edit; auth-ready foundations (hashing utilities, structured config); production-ready Dockerfiles + docker-compose.yml.
- Tech decisions:
  - Backend: Gin v1.11+, GORM v2 with pgx/v5 driver, gin-gonic/contrib middleware (CORS/logging), Viper or godotenv for config, golang.org/x/crypto for hashing utilities.
  - Frontend: Vue 3 + TS + Vite; Tailwind CSS v4 via @tailwindcss/vite; Pinia for state; Axios for HTTP; vue-router for views; optional Headless UI/DaisyUI if needed.
  - DB: PostgreSQL latest stable; auto-migrations via GORM.
  - Containers: Separate services for api, web, db; nginx optional if we want reverse proxy.
- Progress log:
  - DONE: Scaffolded backend (cmd/internal layers, router with health + CORS, config loader with Viper+godotenv, DB connection stub via pgx/GORM), added bcrypt helpers + test, pulled core Go deps.
  - DONE: Implemented Note CRUD layers (GORM repo + service validation + Gin controllers/routes), auto-migrate on startup, and added service/handler tests (go test ./... passing).
- DONE: Bootstrapped frontend foundation (Tailwind v4 plugin wired, Pinia store + Axios client + router scaffold, layout shell + starter views); `npm run build` passing.
- DONE: Built notes UI (list + detail routes with Pinia-backed create/update/delete, validation, and empty/loading/error states; direct navigation fetches single notes); `npm run build` passing.
- DONE: Added multi-stage Dockerfiles (Go API + nginx static frontend) and docker-compose stack with Postgres, healthchecks, and shared env defaults (.env.example).
- TODO: add Makefile/runbook polish for local vs docker workflows and any final QA docs.

### Docker / Compose usage
- Defaults live in `.env.example` (copy to `.env` to override).
- Build + run: `docker compose up --build` (or `docker-compose up --build` if using the standalone binary).
- Stop and clean volumes: `docker compose down -v`.
- Ports: API `http://localhost:8080/healthz`, frontend `http://localhost:4173`, Postgres `localhost:5432`.

### Issues to triage
- CORS failing in Docker Compose (frontend hitting API returns CORS error). Likely missing allowed origin config for containerized hostnames/ports.
