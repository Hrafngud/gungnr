# Gungnr Agent Guide

Purpose
- Keep the workflow simple, predictable, and low-duplication.
- Use this file for process rules, not task lists.

Startup requirement
- Read all `*.md` files in the repo at the beginning of each session.

File responsibilities
- `start.md`: One-screen overview of project scope and the working process.
- `manager.md`: Current state snapshot and any extra instructions for the current iteration.
- `next_task.md`: The single next task, clearly scoped with success criteria.
- `memory.md`: Iteration log and completed-task snapshots to preserve progress.
- Planning files (`overall_plan.md`, `backend_plan.md`, `frontend_plan.md`): Task lists and sequencing only.
- Guidelines (`backend_guidelines.md`, `frontend_guidelines.md`): Architecture and setup rules.

Execution rules
- `deploy.sh` is reference-only; do not modify it.
- Perform the single task in `next_task.md`, then update it with the next task.
- Log each session's work in `memory.md` (not in `next_task.md`).
- Update `manager.md` only when the project state or special instructions change.
- Testing is handled by the user; do not add testing as a next task unless asked.
