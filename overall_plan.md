## Overall Plan and Dependencies

Current status: Replace notes CRUD app with Warp Panel web UI and API.

1) Foundations
- Align env variables for backend/frontend and host integrations.
- Define `.env.example` with GitHub and Cloudflare settings.
- Ensure Docker, cloudflared, and templates directory access on host.

2) Backend First
- Scaffold backend structure and config loader.
- Implement GitHub OAuth auth + allowlist.
- Add integrations for GitHub, Cloudflare, Docker, and cloudflared.
- Build job runner and persistence models.
- Add health endpoints for docker and tunnel checks.

3) Frontend Setup
- Create router, auth store, and base layout.
- Build project wizard flows and job status pages.
- Wire API services with auth handling.

4) Dockerization
- Backend and frontend Dockerfiles (multi-stage).
- Compose services: `db`, `api`, `web`.
- Bind mounts for docker socket, templates dir, and cloudflared config.

5) Polish and Docs
- Update runbook and usage instructions.
- Add Makefile targets for dev and compose.
- QA: backend tests, frontend build, compose up.
