## Frontend Plan

Status: Step 1 complete (theme tokens + AppShell). Page map + routing update complete. View refactor complete (Home, Overview, Host Settings, Networking, GitHub done). Step 2 complete (base components, shared state blocks, toasts, inline feedback). Step 6 complete (onboarding overlays + day-to-day guidance). Step 7 complete (GitHub catalog + Networking data panels).

1) Visual Identity and Theme System
- Define design tokens (zinc palette, accent, borders, shadows, radii, spacing).
- Build flat zinc monochrome surfaces with minimal borders and no gradients/glass.
- Content is free-form and separated by custom `<hr>` elements; no rounded containers.
- Establish typography pairings and scale.
- Standardize animations (page load, staggered reveals, modal transitions).
- Icon set: Iconoir.

2) Reusable Component System
- Build base components with variants: Button, Input, Select, Toggle, Card/Panel, Badge/Status, ListRow, Tooltip, Modal/Sheet, FormSidePanel.
- Standardized loading state (page skeletons + inline spinners).
- Standardized form display via a reusable `FormSidePanel` component for all forms in the application. This component should appear from the right, have an overlay, and scrollable inner content.
- Toasts and inline feedback patterns.
- Implement a universal Select component with a custom dropdown and replace all native selects.

3) Layout and Navigation
- Sidebar-first navigation with expand/collapse behavior.
- Move collapse/expand control into the sidebar only; remove top bar toggle.
- Collapsed state uses icons only (no text labels).
- Top bar for user/session actions and host status summary.
- Footer with product info, links, and build metadata.
- Horizontal margin between content and page edge is minimal (5%).
- Replace the top sidebar logo/title block with a GitHub auth indicator (status + login/logout).
  - Move the existing GitHub auth indicator into the sidebar header slot (no new indicator, just relocate).

4) Routes and Information Architecture
- `/login`: simple two-column layout, brief description left, GitHub auth button right, popup OAuth window.
- `/`: Home with Host Status and Quick Deploy sections.
- `/overview`: in-depth host and container data.
- `/host-settings`: cloudflared and Cloudflare machine setup only.
- `/networking`: Cloudflare-specific status, DNS, tunnel health, ingress.
- `/github`: GitHub configuration (token, org, templates).

5) Page Details
- Home > Host Status: running containers count, jobs (queued/running/finished), machine name, tunnel name, domain, last service deployed, onboarding CTA to Host Settings.
- Home > Quick Deploy:
  - Templates and Services as card grids with repo links and deploy actions; deploy forms should open in the `FormSidePanel`.
  - **Quick Services section improvements:**
    - Add service-specific icons to each service card for visual identification.
    - Add a search bar to filter services by name/description.
    - Make the services container have a fixed height (matching Templates section) with scrollable overflow.
  - **Template forms clarification:**
    - "Create from template" form: Only Project name (required) and Subdomain (required, not optional). Remove Proxy Port and Database Port fields (auto-inferred by backend).
    - "Deploy existing" form: Renamed conceptually to forward ANY localhost service (Docker or not). Fields: Project name (identification), Subdomain (web exposure), Running At (localhost port). Cloudflare-only implementation, no Docker involvement required.
  - **Template repo selector:**
    - Add a repo selector list sourced from backend template catalog.
    - Show multiple templates with repo owner/name + short description.
    - Allow empty state when no templates are configured.
- Overview: container list, job timeline, last activity (remove resources section).
- Host Settings: cloudflared config path, token, tunnel setup status, validation and hints.
  - Move Running containers under Host integrations.
  - Settings forms (e.g., for cloudflared, GitHub) should open in the `FormSidePanel`.
  - The `FormSidePanel` should also contain the status indicators, presented as a compact grid with clamped long values.
  - **Cloudflared ingress preview should use a sidebar panel (like FormSidePanel) for visual cleanup.**
  - Running containers cards: remove tunnel forward input; add Stop/Restart/Remove/Logs actions and a destructive confirmation modal (with a two-step confirmation for volume deletion).
  - Running containers list should also show stopped containers with filters (running/stopped/all).
  - Add iconography to action buttons (stop/restart/remove/logs).
  - Add basic Docker usage summary (disk used, images/containers/volumes count).
  - Add project filters to scope containers/volumes for multi-container templates.
  - Use project-aware labels (compose project/service) for consistent filtering UI.
- Networking:
  - Tunnel status, routing status, Cloudflare health signals.
  - **'Expected DNS records' section should be a 4-column grid for compact information display.**
  - **Ingress preview should use a sidebar panel (like FormSidePanel) for visual cleanup.**
- GitHub: token status, allowlist, templates availability.

6) Journeys
- **NEW Guidance System (replaces overlay-with-highlight):**
  - Remove onboarding overlay approach entirely.
  - Implement contextual form field guidance that appears on form field focus/select.
  - Guidance text: large-font, short, objective explanations.
  - Positioning: appears on the left side of the screen (opposite to the form panel), always in the same fixed position for easy readability.
  - Guidance should appear above the overlay layer.
  - Include external links (GitHub API tokens, Cloudflare settings) when applicable for quick access.
  - Apply to ALL forms throughout the application.
- Day-to-day: focus on quick deploy, status checks, and recent activity.

7) Data and Integration
- Axios base URL and auth handling (401 -> login).
- Map new pages to existing settings/health APIs and add any missing endpoints as needed.
- Add help modals with Cloudflare tunnel setup steps and links to the official docs (config.yml, route dns, run as a service).
- Auto-redirect from login when `/auth/me` succeeds to avoid stuck login state.
- Add template catalog API consumption and repo selector state in Home view.
- Add Docker usage and container status filtering support in Host Settings.
- Add project filtering support for containers/volumes in Host Settings and Overview.
- Add a global loading overlay for page-level loads and critical actions.
- Add iconography to refresh/edit/login/logout actions.
- Redirect to login on logout.
 - Global loading overlay should block page content until initial data load completes (not for every small request).
 - Overlay must not hide error toasts or prevent retry actions when initial load fails.

8) Testing and Quality
- `npm run build` for smoke test.
- Add component tests only if needed.

9) Docker/Build
- Build static assets and serve via nginx in production container.

10) Observability
- Add a live logs screen to stream all running container logs with filters and pause/resume.
- Add a responsive top bar on the logs screen so controls fit across breakpoints.
