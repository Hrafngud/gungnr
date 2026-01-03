## Overall Plan and Dependencies

Current status: Replace notes CRUD app with Warp Panel web UI and API.

1) Foundations
- Align env variables for backend/frontend and host integrations.
- Define `.env.example` with GitHub and Cloudflare settings.
- Ensure Docker, cloudflared, and templates directory access on host.
- Plan for runtime settings in DB (domain, tokens, cloudflared config path).
- Decide host-installed `cloudflared` service as the only tunnel path (no compose cloudflared container).
- Adopt an API-run Docker runner via socket for container operations (no host worker).
- Treat `deploy.sh` as reference-only; do not modify it. The UI should first mirror its CLI behavior before advanced automation.

2) Backend First
- Scaffold backend structure and config loader.
- Implement GitHub OAuth auth + allowlist.
- Add integrations for GitHub, Cloudflare, Docker, and cloudflared.
- Build job runner and persistence models.
- Add health endpoints for docker and tunnel checks.
- Add settings endpoints and use them in workflows (domain/token/config path).
- Add Docker runner job types for `docker run` and `docker compose` from the API.
- Add container lifecycle controls (stop/restart/remove) with clear stop-vs-remove semantics.
- Add automatic container naming for multiple instances of the same image (suffix with incrementing numbers).
- Persist onboarding state per user to avoid repeated overlays.

3) Frontend Setup
- Create router, auth store, and base layout.
- Build component system with variants, loading states, and animations.
- Build sidebar navigation, top bar, and footer.
- Implement Home, Overview, Host Settings, Networking, and GitHub pages.
- Rework Host Settings layout: side panels for settings and ingress preview, and a slimmer status grid.
- Update running container cards with stop/restart/remove/logs actions and confirmation flow.
- Remove the Overview Resources section.
- Wire API services with auth handling.
- Add onboarding overlay journey and day-to-day flow polish.
- Improve sidebar collapse UX (icon-only, toggle in sidebar only).
- Replace native selects with a universal custom Select component.
- Reduce horizontal padding/margins to maximize content width.
- Fix login page layout and auto-redirect on `/auth/me` success.
- Refactor Home Quick Deploy into card grids for Templates/Services with repo links and deploy actions.
- Add a responsive top bar to the logs screen so controls fit on all widths.
- **UX Refinement Phase:**
  - Enhance Quick Services with icons, search bar, and fixed-height scrollable container.
  - Replace onboarding overlay system with contextual form field guidance (focus-triggered, positioned left, with external links).
  - Create ingress preview sidebar component for Networking and Host Settings visual cleanup.
  - Convert Networking DNS records to 4-column grid layout.
  - Simplify template forms: "Create from template" (project name + subdomain only, auto-infer ports), "Deploy existing" (forward ANY localhost service via Cloudflare-only, no Docker required).

4) Dockerization
- Backend and frontend Dockerfiles (multi-stage).
- Compose services: `db`, `api`, `web`.
- Bind mounts for docker socket, templates dir, and cloudflared config.

5) Polish and Docs
- Update runbook and usage instructions.
- Add Makefile targets for dev and compose.
- QA: backend tests, frontend build, compose up.

6) Observability
- Add live container logs screen for all running containers (not just deploy jobs).
- Provide filtering by container name and stream logs via SSE or WebSocket.
