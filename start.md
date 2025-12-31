Workflow: Build a Dockerized "Warp Panel" web UI to perform all deploy.sh tasks from a browser.
deploy.sh is reference-only; do not modify it. The UI must mirror its CLI behavior (local tunnel setup + deploy flows) before any advanced automation.
Stack: Go backend + Vue 3 TS frontend + PostgreSQL, fully containerized via docker compose.
The app runs on the host PC and controls local resources (Docker, filesystem, cloudflared).
It is also accessible remotely via the existing Cloudflare tunnel.

Planning files in root:
manager.md: Project overview and tracker.
backend_guidelines.md / frontend_guidelines.md: Architecture and setup.
backend_plan.md / frontend_plan.md / overall_plan.md: Tasks and sequencing.
next_task.md: Current task.

Process:
Read all .md files before coding.
Execute next_task.md fully.
Testing is handled by the user. Do not assign testing/verification as "Next up" tasks unless explicitly requested.
Document changes, decisions, and results in relevant .md files.
Update next_task.md with the next pending task.
Next task: Clean legacy host-worker data (host_deploy jobs/tokens) and decide on DB schema cleanup.
