## Frontend Plan

Status: UI cleanup and rewiring only to align with bootstrap-first setup.

1) Setup Assumptions
- Assume OAuth login works and tunnel/DNS are already configured by bootstrap.
- Do not introduce new backend dependencies or flows.

2) Cleanup and Rewiring
- Remove or hide UI paths that imply missing Cloudflare or OAuth setup.
- Keep Host Settings focused on inspection, validation, and minor adjustments.
- Present GitHub App configuration in a clearly scoped section labeled for "Create from template" only.
- Disable template creation UI when GitHub App settings are missing and show concise guidance.

3) Preserve Existing Behavior
- Do not alter job system, logs, or networking behavior.
- Keep RBAC gating as-is.
