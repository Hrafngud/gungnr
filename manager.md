## Warp Panel - Project State

Scope
- Web control panel to perform all deploy.sh tasks from a browser (deploy.sh is reference-only).
- Runs on the host PC (Docker, filesystem, cloudflared access) and is accessible remotely via Cloudflare Tunnel.

Current state
- UX overhaul complete: dark zinc mono theme, expanded layout, sidebar-first navigation, custom components, consistent animations.
- Core flows wired: create from template, deploy existing, forward local service, tunnel/DNS updates, Docker lifecycle controls.
- GitHub App flow implemented end-to-end for template generation (App credentials, installation token minting, create-from-template).
- Host Settings reworked with form side panels, ingress preview sidebar, and Docker usage/filters.

Active decisions
- Prefer API-run Docker socket runner; host-worker flow is retired.
- Cloudflared runs on host (no compose cloudflared service).

Additional instructions
- None for this iteration.
