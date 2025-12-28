## Overall Plan & Dependencies

Current status: Backend and frontend CRUD flows are in place and containerized (API + nginx frontend + Postgres via compose); next focus is docs/polish.

1) Foundations
- Align env variables for backend/frontend Docker; define `.env.example` (API port, DB URL, frontend base URL).
- Confirm Go/Vue toolchains installed locally.

2) Backend First
- Scaffold backend structure and config loader.
- Add dependencies (Gin, GORM+pgx, Viper, etc.) and implement DB connection + Note model.
- Build repository/service/controllers + routes; add healthcheck.
- Write minimal tests; ensure `go test ./...` passes.

3) Frontend Setup
- Install Tailwind v4 plugin, Pinia, Axios, vue-router; wire Vite config.
- Create layout, router, store, and services; connect to API once backend ready.

4) Dockerization
- Backend Dockerfile (multi-stage), frontend Dockerfile (build → nginx/static).
- Compose with services: `db` (Postgres), `api`, `web`; set healthchecks and network.
- Optional nginx reverse proxy for unified hostnames.

5) Polish & Docs
- Update README/manager.md with run commands.
- Add Makefile scripts for local dev and docker-compose usage.
- Final QA: run backend tests, frontend build, compose up check.
