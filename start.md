Workflow: Build a Dockerized "Warp Panel" web UI to perform all deploy.sh tasks from a browser.
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
Test thoroughly: run docker compose up --build, verify functionality, fix errors.
Only mark task complete if build and app work perfectly.
Document changes, decisions, and results in relevant .md files.
Update next_task.md with the next pending task.
