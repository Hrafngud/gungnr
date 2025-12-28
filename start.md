Workflow: Build a Dockerized CRUD notes app (title + content) with Go backend + Vue 3 TS frontend + PostgreSQL.
Tech stack (Dec 2025):

Backend: Gin v1.11+, GORM v2, pgx driver.
Frontend: Vue 3 + Vite + TS + Tailwind CSS v4 + Pinia v3 + Axios.
Fully containerized via docker-compose.

Planning files in root:

manager.md: Project overview/tracker.
backend_guidelines.md / frontend_guidelines.md: Architecture & setup.
backend_plan.md / frontend_plan.md / overall_plan.md: Tasks.
next_task.md: Current task.

Process:

Read all .md files.
Execute next_task.md fully.
Test thoroughly: run docker compose up --build, verify functionality, fix errors.
Only mark task complete if build and app work perfectly.
Document all changes, decisions, and results in relevant .md files (especially manager.md and plans).
Update next_task.md with the next pending task.
