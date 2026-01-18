## Overall Plan and Dependencies

Current status: Pivot to bootstrap-first installation; backend API and DB schema are frozen.
Guiding principle: one-time terminal bootstrap that leaves the panel fully wired on first run.

1) Installer Script (install.sh)
- Add `install.sh` at repo root.
- Detect OS/arch and install the `gungnr` CLI binary into `/usr/local/bin/gungnr`.
- Verify/install Docker, Docker Compose, and cloudflared.
- Do not write config files or start services.
- Print: `Run "gungnr bootstrap" to configure this machine.`

2) Gungnr CLI (Go binary)
- Implement `gungnr bootstrap` as the only setup command.
- Inspect environment and abort if an existing Gungnr install is detected.
- Use GitHub device flow to identify and seed the SuperUser.
- Run `cloudflared tunnel login`, create a tunnel, generate a full config.yml, and start the tunnel as a user-managed process.
- Prompt for domain + Cloudflare API token, validate scopes, and create DNS routing.
- Materialize filesystem paths and generate a complete `.env` (no placeholders).
- Start Docker Compose, wait for API health, and print the panel URL.

3) UI Cleanup (no backend changes)
- Remove/hide paths that imply missing Cloudflare or OAuth setup.
- Keep Host Settings focused on inspection/validation and minor adjustments.
- Label GitHub App settings as required only for "Create from template."
- Disable template creation when GitHub App settings are missing.
- Preserve existing deploy, logs, and RBAC behavior.

4) Docs/Runbook Alignment
- Update README/process docs to describe install.sh + `gungnr bootstrap`.
- Keep `deploy.sh` reference-only.
