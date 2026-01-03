## Next Task

UX refinement phase: Improve Quick Services UI, redesign guidance system, add ingress preview sidebar, and clarify template deployment workflows.

Progress tracking rules (mandatory for each new Codex session):
- Read this file first and append a new entry under "Iteration log".
- Update checkbox status for any task you touched.
- Add brief notes for decisions, command outputs, or blockers.
- Keep "Next up" accurate (1-3 bullets).
- Do not list testing or verification as a "Next up" task; the user handles testing and feedback.

Task breakdown (checklist):
- [x] UI-REWORK-1: All pages content: Remove rounded containers, reduce horizontal margins to ~5%, and use custom `<hr>` separators.
- [x] UI-REWORK-2: All application forms: Create a shared `FormSidePanel` component for all forms, with a right-side slide-in animation and overlay.
- [x] UI-REWORK-3: Refactor the first form to use the new `FormSidePanel`.
- [x] UI-REWORK-4: Refactor all remaining forms to use `FormSidePanel` (HomeView deploy existing, HomeView quick service, HostSettingsView settings).
- [x] UI-REWORK-5: Refactor HomeView to a more concise, componentized approach. Break down the large view into smaller, reusable components (e.g., HostStatusPanel, TemplateCardsSection, ServiceCardsSection).
- [x] UI-REWORK-6: Standardize sidebar animations for both navigation and form sidebars. Create smooth, consistent open/close animations with proper timing and easing.
- [x] UX-REFINE-1: Quick Services improvements - Add service icons, search bar, fixed-height scrollable container matching Templates section height.
- [ ] UX-REFINE-2: Redesign guidance system - Replace overlay-with-highlight with large-font contextual help on form field focus, positioned on left side opposite form, with external links (GitHub, Cloudflare) when applicable.
- [ ] UX-REFINE-3: Ingress preview sidebar - Convert live ingress preview to a sidebar panel (like FormSidePanel) for visual cleanup in Networking and Host Settings.
- [x] UX-REFINE-4: Networking DNS grid - Convert 'Expected DNS records' section to a 4-column grid layout for compact info display.
- [x] UX-REFINE-5: Template form clarification - Update 'Create from template' form to only include: Project name (required), Subdomain (required, not optional). Remove Proxy/DB port fields (auto-inferred).
- [x] UX-REFINE-6: Deploy existing clarification - Update 'Deploy existing' form to forward ANY localhost service (Docker or not) with fields: Project name (identification), Subdomain (web exposure), Running At (localhost port). Cloudflare-only, no Docker involvement.

Iteration log (append each session):
- 2026-01-02: User initiated a major visual rework. Pivoting from previous tasks.
- 2026-01-03 (Session 1): Starting UI-REWORK-1 - removing rounded containers, reducing margins, adding custom hr separators.
  - Completed UI-REWORK-1: Removed border-radius from all panel, list-row, state, modal, toast, and onboarding classes in style.css. Horizontal margins already at 5% (`px-[5%]` in AppShell.vue). Added `<hr />` separators to HomeView, OverviewView, ActivityView, GitHubView, HostSettingsView, JobsView, and NetworkingView.
  - Completed UI-REWORK-2: Created UiFormSidePanel component with right-side slide-in animation, overlay, scrollable content, and close button.
  - Completed UI-REWORK-3: Refactored template creation form in HomeView to use the new FormSidePanel component.
- 2026-01-03 (Session 2): Completing UI-REWORK-4 - refactoring all remaining forms.
  - Refactored HomeView: All three forms (template creation, deploy existing, quick service) now use FormSidePanel component with proper state management.
  - Refactored HostSettingsView: Settings form now uses FormSidePanel with status indicators included in the panel.
  - All forms in the application now consistently use the FormSidePanel approach.
- 2026-01-03 (Session 2 continued): User requested additional UI rework tasks.
  - Added UI-REWORK-5: Componentize HomeView for better maintainability and reusability.
  - Added UI-REWORK-6: Standardize sidebar animations across navigation and form sidebars.
- 2026-01-03 (Session 3): Completed UI-REWORK-5 and UI-REWORK-6.
  - Created three new components in /components/home/: HostStatusPanel.vue, TemplateCardsSection.vue, ServiceCardsSection.vue.
  - Refactored HomeView.vue to use the new components, reducing it from 1204 lines to 775 lines (35.6% reduction).
  - Added smooth sidebar animations to AppShell.vue with consistent timing (0.3s ease for width, 0.25s ease for opacity).
  - Standardized all sidebar and modal transitions: navigation sidebar (0.3s), form sidebars (0.3s), modal overlay (0.25s).
  - Frontend visual rework is now complete.
- 2026-01-03 (Session 4): Planning phase for UX refinement cycle.
  - User reviewed deploy.sh script context and identified UX improvements needed.
  - Defined 6 new UX-REFINE tasks focusing on: Quick Services polish, contextual guidance system, ingress preview sidebar, DNS grid layout, and template form simplification.
  - Key insight: "Deploy existing" should forward ANY localhost service (not just Docker) via Cloudflare-only approach.
  - Key insight: "Create from template" should auto-infer ports and require only project name + subdomain.
  - Backend planning for GitHub/Templates integration deferred for future discussion.
- 2026-01-03 (Session 5): Completed UX-REFINE-1, UX-REFINE-4, UX-REFINE-5, and UX-REFINE-6.
  - UX-REFINE-1: Quick Services improvements.
    - Added icon field to ServicePreset type and assigned icons to all 20 service presets (draw, ai, database, cache, server, storage, mail, tool, admin, code, git, search).
    - Implemented search bar with icon for filtering services by name/description.
    - Added service-specific icons to each card with proper icon mapping using inline SVG paths.
    - Set fixed-height scrollable container (368px) with empty state message for no search results.
    - Quick Services section now has consistent visual hierarchy and improved discoverability.
  - UX-REFINE-5: Simplified "Create from template" form.
    - Removed Proxy Port and Database Port fields (now auto-inferred by backend).
    - Made Subdomain field required instead of optional.
    - Updated form description to clarify automatic port configuration.
    - Added field-level help text explaining each input's purpose.
  - UX-REFINE-6: Clarified "Deploy existing" form (renamed to "Forward localhost service").
    - Renamed from "Deploy existing" to "Forward localhost service" to better reflect purpose.
    - Updated to forward ANY localhost service (Docker or not) via Cloudflare-only approach.
    - Changed fields: Service name (required), Subdomain (required), Running at (required localhost port).
    - Removed Docker-specific UI elements (local project list, template folder references).
    - Updated template card descriptions to match new form purposes.
  - UX-REFINE-4: Converted DNS records to 4-column grid layout.
    - Replaced vertical UiListRow layout with compact 4-column grid (Subdomain, Full hostname, Type, Target service).
    - Added header row with column labels.
    - Each DNS record now displays as a single row with all information visible at a glance.
    - Applied truncate to prevent long values from breaking the layout.

Next up (keep this short):
- Create ingress preview sidebar component for Networking/Host Settings cleanup.
- Replace onboarding overlay system with contextual form field guidance.
