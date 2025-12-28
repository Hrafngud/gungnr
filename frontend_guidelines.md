## Frontend Guidelines (Vue 3 + TS + Vite)

### Stack
- Vue 3 + TypeScript + Vite (existing).
- Tailwind CSS v4 via `@tailwindcss/vite` plugin (config-light, faster builds).
- State: Pinia (TS-friendly).
- HTTP: Axios with interceptors.
- Routing: `vue-router@4`.
- Optional UI helpers: Headless UI (Vue) or DaisyUI for Tailwind components if we need primitives.

### Dependency Install (from `frontend/go-notes`)
```bash
cd frontend/go-notes
npm install -D tailwindcss @tailwindcss/vite
npm install pinia vue-router@4 axios
# If using Headless UI + transitions:
# npm install @headlessui/vue @heroicons/vue
```

### Tailwind v4 Setup (latest 2025)
- Vite plugin integration:
```ts
// vite.config.ts
import tailwindcss from '@tailwindcss/vite'
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  plugins: [vue(), tailwindcss()],
})
```
- Entry stylesheet (e.g., `src/assets/main.css`): ensure the new v4 layers are present:
```css
@import "tailwindcss/base";
@import "tailwindcss/components";
@import "tailwindcss/utilities";
```
- Tailwind v4 ships with a default design system; add `tailwind.config.ts` only if customizing theme/plugins. Example scaffold:
```ts
import type { Config } from 'tailwindcss'
export default {
  content: ['./index.html', './src/**/*.{vue,ts,tsx}'],
  theme: { extend: {} },
  plugins: [],
} satisfies Config
```

### Project Conventions
- Use `<script setup lang="ts">`.
- Organize by feature: `components/`, `views/`, `stores/`, `services/api.ts`.
- Axios instance with base URL from `import.meta.env.VITE_API_BASE_URL`; set JSON headers, handle errors centrally.
- Pinia stores handle async actions; components stay lean.
- Use composables for cross-cutting concerns (e.g., `useNotifications`, `useNotesApi`).

### UI/UX Notes
- Layout: responsive split list/detail or simple list + modal.
- Forms: accessible labels, required indicators, error text.
- Loading/empty/error states for list and detail.
- Keep Tailwind classes readable; extract to small components when verbose.

### Testing & Quality
- Lightweight checks: `npm run build`, `npm run lint` (if ESLint configured).
- Component tests (optional) with Vitest + Vue Test Utils.

### Run Scripts
```bash
cd frontend/go-notes
npm run dev
npm run build
```
- For Docker: build static assets, serve via nginx or `vite preview` behind container entrypoint.
