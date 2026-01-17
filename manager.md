## Gungnr - Project State

Scope
- Web control panel to perform all deploy.sh tasks from a browser (deploy.sh is reference-only).
- Runs on the host PC (Docker, filesystem, cloudflared access) and is accessible remotely via Cloudflare Tunnel.

Current state
- Planning pivot to a bootstrap-first setup (installer + CLI) with UI cleanup only.
- Backend API and DB schema are frozen for this iteration.
- Existing UI/flows remain functional; cleanup will remove setup-oriented guidance.

Active decisions
- Prefer API-run Docker socket runner; host-worker flow is retired.
- Cloudflared runs on host (no compose cloudflared service).
- Bootstrap is mandatory at install time; no optional setup paths.
- GitHub App configuration remains inside the UI and is scoped to template creation only.

Additional instructions
- None for this iteration.
