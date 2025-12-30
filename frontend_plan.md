## Frontend Plan

Status: Step 1 complete (theme tokens + AppShell). Page map + routing update complete. View refactor complete (Home, Overview, Host Settings, Networking, GitHub done). Step 2 complete (base components, shared state blocks, toasts, inline feedback). Step 6 complete (onboarding overlays + day-to-day guidance). Step 7 complete (GitHub catalog + Networking data panels).

1) Visual Identity and Theme System
- Define design tokens (zinc palette, accent, borders, shadows, radii, spacing).
- Build flat zinc monochrome surfaces with minimal borders and no gradients/glass.
- Establish typography pairings and scale.
- Standardize animations (page load, staggered reveals, modal transitions).
- Icon set: Iconoir.

2) Reusable Component System
- Build base components with variants: Button, Input, Select, Toggle, Card/Panel, Badge/Status, ListRow, Tooltip, Modal/Sheet.
- Standardized loading state (page skeletons + inline spinners).
- Standardized form and info display pages via composables with explicit states (loading, empty, error, ready).
- Toasts and inline feedback patterns.

3) Layout and Navigation
- Sidebar-first navigation with expand/collapse behavior.
- Top bar for user/session actions and host status summary.
- Footer with product info, links, and build metadata.

4) Routes and Information Architecture
- `/login`: simple two-column layout, brief description left, GitHub auth button right, popup OAuth window.
- `/`: Home with Host Status and Quick Deploy sections.
- `/overview`: in-depth host and container data.
- `/host-settings`: cloudflared and Cloudflare machine setup only.
- `/networking`: Cloudflare-specific status, DNS, tunnel health, ingress.
- `/github`: GitHub configuration (token, org, templates).

5) Page Details
- Home > Host Status: running containers count, jobs (queued/running/finished), machine name, tunnel name, domain, last service deployed, onboarding CTA to Host Settings.
- Home > Quick Deploy: Templates (with GitHub link + quick deploy) and Services (Excalidraw, OpenWebUI, Ollama, Redis, Postgres, etc).
- Overview: container list, job timeline, resource snapshots, last activity.
- Host Settings: cloudflared config path, token, tunnel setup status, validation and hints.
- Networking: tunnel status, DNS records, routing status, Cloudflare health signals.
- GitHub: token status, allowlist, templates availability.

6) Journeys
- Onboarding: guided overlay with field highlights, step-by-step instructions, and links to API key creation.
- Day-to-day: focus on quick deploy, status checks, and recent activity.
  - Status: Onboarding overlays cover Home, Host Settings, Networking, and GitHub with API key links; day-to-day callouts added to Quick Deploy, Overview Activity, Jobs, and Activity.

7) Data and Integration
- Axios base URL and auth handling (401 -> login).
- Map new pages to existing settings/health APIs and add any missing endpoints as needed.

8) Testing and Quality
- `npm run build` for smoke test.
- Add component tests only if needed.

9) Docker/Build
- Build static assets and serve via nginx in production container.
