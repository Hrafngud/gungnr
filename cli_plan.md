# CLI Restructure + Bubbletea TUI Plan

Goals
- Keep the existing bootstrap behavior and flow unchanged.
- Split the CLI into clear layers (inputs, output, integrations, validation).
- Deliver a polished, engaging TUI using Bubbletea.

Constraints
- Backend API and DB schema remain frozen.
- Logic remains the same; only structure, readability, and UX improve.
- Continue to ship a single Go binary.

References
- Bubbletea model lifecycle (Init/Update/View) from the official tutorial.

Proposed package layout
- cmd/gungnr/main.go: CLI entry, argument parsing, runtime selection.
- internal/cli/app: bootstrap runner, step orchestration, shared state.
- internal/cli/tui: Bubbletea program, views, key bindings, styles.
- internal/cli/prompts: input collection, defaults, validation wrappers.
- internal/cli/strings: standardized messages, links, and help text.
- internal/cli/ui: formatting helpers, colors, consistent status output.
- internal/cli/validate: focused validation utilities.
- internal/cli/flow: non-TUI flow runner (text mode, for CI or fallback).
- internal/cli/integrations:
  - github: device flow + user lookup.
  - cloudflare: API, DNS, token validation.
  - cloudflared: tunnel login/create/run/status.
  - docker: docker + compose checks, start stack.
  - filesystem: paths, env file, cloudflared config.

Step-based bootstrap runner
- Define a small step interface in internal/cli/app:
  - ID(), Title(), Run(ctx, state, deps, ui) -> StepResult.
- Each step emits structured events (started, progress, prompt, success, error).
- State collects all output data (GitHub identity, tunnel info, env paths, etc.).
- Errors are typed as recoverable vs fatal to drive UI behavior.

TUI experience plan (Bubbletea)
- Use tea.Model with Init/Update/View as per docs.
- Adopt Bubbles for polish:
  - spinner for long operations (cloudflared, DNS checks, health checks).
  - progress for overall step progress.
  - textinput for prompts (domain, tokens, callback URL).
  - viewport for scrollable logs/help when content exceeds screen.
  - help for key bindings (quit, back, retry, copy).
- Screen flow (single model with subviews or separate models):
  1) Welcome + preflight checks (live checklist).
  2) GitHub device flow screen (code + link + status).
  3) Cloudflared login + tunnel create (progress + output).
  4) DNS configuration prompts + validation feedback.
  5) Env generation + compose start (progress + spinner).
  6) Final summary screen with paths, URLs, and next steps.
- Status messaging should be standardized via internal/cli/strings and styled with lipgloss.

Input and prompt decoupling
- Centralize prompt logic in internal/cli/prompts to support both TUI and fallback.
- Provide reusable input validators (non-empty, domain format, token cleanup).
- Allow TUI to present the same help text and links as current bootstrap output.

Standardized messaging
- Create a message catalog for repeated guidance:
  - GitHub OAuth app link.
  - Cloudflare token page + required scopes.
  - Clarify that cloudflared runs as the current user.
- Build helpers that format info, warning, and error blocks consistently.

Error handling strategy
- Define a small error type with:
  - User message.
  - Optional remediation hint.
  - Optional raw error for logs.
- TUI renders friendly errors with a retry option; fallback mode prints both.

Migration plan (implementation sequence)
1) Extract the current bootstrap logic into internal/cli/app and integrations.
2) Introduce state structs (paths, tunnel info, dns info, env, summary).
3) Add the prompt and validation helpers to replace inline input logic.
4) Introduce standardized messages/links and route current copy through them.
5) Create a non-TUI runner that mirrors current output (baseline parity).
6) Implement Bubbletea model and screens, wire into the runner events.
7) Keep cmd/gungnr/main.go small; select TUI by default with a fallback flag.
8) Ensure output parity and final summary matches current bootstrap results.

Success criteria
- The bootstrap flow outputs the same data and performs the same side effects.
- CLI is split into coherent packages with minimal cross-coupling.
- TUI presents a clear, step-based, interactive flow with helpful guidance.
