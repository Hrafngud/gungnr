## Frontend Plan

Status: Notes UI implemented (list/detail layout, Pinia CRUD actions wired to API, validation + loading/empty/error states). Dockerized with nginx static container; next: final polish/docs.

1) Dependencies & Base Setup
- Add Tailwind v4 plugin in Vite config; ensure main stylesheet imports tailwind layers.
- Install Pinia, Axios, vue-router; create `src/stores/index.ts` and router scaffold.
- Configure Axios instance with base URL env + interceptors.

2) Project Structure
- `src/components/` for reusable UI (buttons, inputs, modal).
- `src/views/` for pages (NotesList, NoteDetail/Edit).
- `src/stores/notes.ts` for note state/actions.
- `src/services/api.ts` for Axios client; `src/services/notes.ts` for API wrappers.
- `src/types/note.ts` for shared types.

3) Layout & Routing
- Basic routes: `/` → Notes list, `/notes/:id` for detail/edit (or modal).
- Global shell with header + responsive container.

4) Note Features
- List notes with created/updated timestamps.
- Create note form (title/content/tags).
- Edit and delete flows; confirm delete.
- Empty/loading/error states.

5) UI/UX Enhancements
- Toast/inline alerts for success/error.
- Keyboard-accessible modals/forms.
- Tailwind utility classes; extract common styles when verbose.

6) State & Data Flow
- Pinia actions calling service layer; keep optimistic updates optional.
- Cache last fetched list; invalidate on create/update/delete.

7) Testing / Quality
- Smoke test components with Vitest (if configured) and run build.
- Type-check via `vue-tsc --noEmit` if available.

8) Docker/Build
- Ensure `npm run build` outputs to `dist/`. DONE
- Add Dockerfile to build assets and serve via nginx or `vite preview` in production container. DONE (nginx + SPA fallback)
