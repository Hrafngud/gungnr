## Frontend Plan

Status: Replace notes UI with Warp Panel control UI.

1) Dependencies and Base Setup
- Ensure Tailwind v4 via `@tailwindcss/vite`.
- Install Pinia, Axios, vue-router.
- Configure Axios base URL and auth handling (401 -> login).

2) Project Structure
- `src/components/` for form controls, status chips, log viewer.
- `src/views/` for Login, Dashboard, Projects, Jobs, Settings.
- `src/stores/` for auth, projects, jobs.
- `src/services/` for API wrappers.
- `src/types/` for DTOs.

3) Routes and Layout
- `/login` for GitHub OAuth trigger.
- `/` dashboard view.
- `/projects` list and deploy flows.
- `/jobs/:id` detail with log viewer.
- `/settings` for config/health hints.

4) Core Flows
- Create from template wizard with validation.
- Deploy existing project flow.
- Quick local service flow.
- Job status and log streaming (polling or SSE).

5) UX Polish
- Toasts for success/error.
- Clear empty states and call-to-action buttons.
- Form validation for subdomain and ports.

6) Testing and Quality
- `npm run build` for smoke test.
- Add component tests only if needed.

7) Docker/Build
- Build static assets and serve via nginx in production container.
