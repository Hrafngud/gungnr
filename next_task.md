## Next Task

Objective
Add project filter support to the host containers list endpoint so the UI can scope running/stopped containers to a compose project.

Success criteria
- `/api/v1/host/containers` accepts an optional `project` query param and returns only containers whose compose project label matches (case-insensitive).
- Invalid project values return a 400 with a clear error message.
- Container responses still include both running and stopped items when filtered, matching current behavior.

Notes
- Keep task lists in planning files only. Do not move tasks back into manager.md.
