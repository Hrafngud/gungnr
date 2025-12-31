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
- Implement a universal Select component with a custom dropdown and replace all native selects.

3) Layout and Navigation
- Sidebar-first navigation with expand/collapse behavior.
- Move collapse/expand control into the sidebar only; remove top bar toggle.
- Collapsed state uses icons only (no text labels).
- Top bar for user/session actions and host status summary.
- Footer with product info, links, and build metadata.
- Reduce horizontal padding/margins to give more space to main page content.

4) Routes and Information Architecture
- `/login`: simple two-column layout, brief description left, GitHub auth button right, popup OAuth window.
- `/`: Home with Host Status and Quick Deploy sections.
- `/overview`: in-depth host and container data.
- `/host-settings`: cloudflared and Cloudflare machine setup only.
- `/networking`: Cloudflare-specific status, DNS, tunnel health, ingress.
- `/github`: GitHub configuration (token, org, templates).

5) Page Details
- Home > Host Status: running containers count, jobs (queued/running/finished), machine name, tunnel name, domain, last service deployed, onboarding CTA to Host Settings.
- Home > Quick Deploy: Templates and Services as card grids with repo links and deploy actions; show deploy forms only when a card is selected.
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
- Add host-worker flow UI: after deploy requests, show a modal with the exact host command, copy-to-clipboard, and status polling.
- Add help modals with Cloudflare tunnel setup steps and links to the official docs (config.yml, route dns, run as a service).
- Auto-redirect from login when `/auth/me` succeeds to avoid stuck login state.

8) Testing and Quality
- `npm run build` for smoke test.
- Add component tests only if needed.

9) Docker/Build
- Build static assets and serve via nginx in production container.

10) Observability
- Add a live logs screen to stream all running container logs with filters and pause/resume.
- Add a responsive top bar on the logs screen so controls fit across breakpoints.
