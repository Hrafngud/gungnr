## Frontend Guidelines (Vue 3 + TS + Vite)

### Stack
- Vue 3 + TypeScript + Vite.
- Tailwind CSS v4 via `@tailwindcss/vite`.
- State: Pinia.
- HTTP: Axios with interceptors.
- Routing: `vue-router@4`.

### Project Conventions
- Use `<script setup lang="ts">`.
- Organize by feature: `components/`, `views/`, `stores/`, `services/api.ts`.
- Axios instance with base URL from `import.meta.env.VITE_API_BASE_URL`.
- Keep views thin; move logic into Pinia stores and composables.

### UX Requirements
- Auth gate: GitHub login button, show user avatar/login when authenticated.
- Dashboard: list existing template projects with status (running/stopped), ports, and actions.
- Wizards:
  - Create from template (name, subdomain, ports auto or override).
  - Deploy existing project (select project, subdomain, host port).
  - Quick local service (subdomain, port).
- Job status: show progress and logs (poll or SSE/websocket).
- Settings view: show config hints and health checks (tunnel status, docker status).

### UI/UX Notes
- Clear success/error toasts for each action.
- Loading and empty states for lists and job logs.
- Form validation with helpful hints for ports and subdomains.

### Testing and Quality
- Smoke test with `npm run build`.
- Optional component tests via Vitest if configured.

### Run Scripts
```bash
cd frontend
npm run dev
npm run build
```
