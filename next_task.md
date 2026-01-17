## Next Task

Objective
Add a repo-root `install.sh` that installs the Gungnr CLI and verifies prerequisites.

Success criteria
- `install.sh` detects OS/arch and installs the `gungnr` CLI to `/usr/local/bin/gungnr`.
- The script verifies Docker, Docker Compose, and cloudflared are present (installing via package manager when possible, otherwise failing with clear errors).
- The script does not write config files or start services.
- The script ends with: `Run "gungnr bootstrap" to configure this machine.`

Notes
- Keep task lists in planning files only. Do not move tasks back into manager.md.
