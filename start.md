Workflow: Build a Dockerized "Gungnr" web UI plus a bootstrap CLI to make setup one-command.
deploy.sh is reference-only; do not modify it. During the bootstrap rework, the backend API and DB schema are frozen.
Stack: Go backend + Vue 3 TS frontend + PostgreSQL, fully containerized via docker compose.
The app runs on the host PC and controls local resources (Docker, filesystem, cloudflared).
It is also accessible remotely via a Cloudflare tunnel configured during bootstrap.

Core docs
- AGENTS.md: Process rules and file responsibilities.
- manager.md: Current state snapshot + any special instructions for the current iteration.
- next_task.md: Single next task with success criteria.
- memory.md: Iteration history and completed-task snapshots.
- backend_guidelines.md / frontend_guidelines.md: Architecture and setup.
- backend_plan.md / frontend_plan.md / overall_plan.md: Task lists and sequencing only.

Process
- Read all `*.md` files before coding.
- Perform the single task in next_task.md fully.
- Testing is handled by the user; do not assign testing/verification as the next task unless requested.
- Document changes, decisions, and results in memory.md.
- Update next_task.md with the next pending task.
