## Frontend Guidelines (Vue 3 + TS + Vite)

### Stack
- Vue 3 + TypeScript + Vite.
- Tailwind CSS v4 via `@tailwindcss/vite`.
- State: Pinia.
- HTTP: Axios with interceptors.
- Routing: `vue-router@4`.

### Project Conventions
- Use `<script setup lang="ts">`.
- Organize by feature and layer: `components/` (base, layout, domain), `views/`, `stores/`, `composables/`, `services/api.ts`.
- Axios instance with base URL from `import.meta.env.VITE_API_BASE_URL`.
- Keep views thin; move logic into Pinia stores and composables.

### Visual Identity
- Dark zinc palette with flat monochrome surfaces; avoid gradients, glass, and heavy elevation.
- Use a delicate turquoise accent for CTAs and key focus states while keeping surfaces monochrome.
- Provide semantic status colors (success, warning, danger) for badges, toasts, and indicators.
- Define theme tokens as CSS variables and reuse across components.
- Icon set: Iconoir.

### Layout and Navigation
- Sidebar-based navigation with expand/collapse behavior (icon-only collapsed state).
- Sidebar toggle control lives in the sidebar only (remove top-bar toggle).
- Top bar for session controls and quick host status.
- Footer with product info and helpful links.

### Auth and Login
- Unauthenticated users see only the login page.
- Login layout: two columns, brief product description on the left, GitHub auth button on the right.
- GitHub auth uses a popup window rather than full-page redirect.
- Auto-redirect to the app when `/auth/me` succeeds (do not leave the user on the login screen).

### Pages and Responsibilities
- Home: Host Status (containers count, jobs status, machine name, tunnel name, domain, last service deployed, onboarding CTA) and Quick Deploy (Templates + Services).
- Overview: in-depth host + container data and recent activity.
- Host Settings: cloudflared and Cloudflare machine setup only.
- Networking: Cloudflare status, DNS, tunnel health, ingress.
- GitHub: token status, allowlist, and template availability.

### UX Requirements
- `deploy.sh` is reference-only; do not modify it. The UI should mirror its CLI behavior before advanced automation.
- Standardized component variants (buttons, cards, badges, inputs).
- Standardized loading state (page skeletons + inline spinners).
- Standardized form and info display pages via composables with states (loading, empty, error, ready).
- Onboarding journey with overlay highlights and step-by-step guidance, including API key links.
- Day-to-day journey focused on quick deploy, status checks, and recent activity.
- Use a custom Select component (no native `<select>` in the UI).
- Present Templates and Services as card grids with clear deploy actions and repo links; only show deploy forms when a card is selected.

### UI/UX Notes
- Clear success/error toasts for each action.
- Loading and empty states for lists and job logs.
- Form validation with helpful hints for ports and subdomains.
 - Standardized animations (page load, staggered reveals, modal transitions).
- Prefer wide content layouts: minimize horizontal padding/margins so main panels use more of the viewport.

### Testing and Quality
- Smoke test with `npm run build`.
- Optional component tests via Vitest if configured.

### Run Scripts
```bash
cd frontend
npm run dev
npm run build
```
