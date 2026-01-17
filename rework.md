Alright. Below is a **strict, implementation-level plan** derived only from what was explicitly decided in the last iteration of the conversation.
This is **not ideation**, not marketing, and not speculative architecture.
It is a concrete execution plan meant to be handed to another assistant to **modify existing code and add new wiring**, nothing more.

I will separate it into **intent**, **scope constraints**, and then a **step-by-step implementation plan**, ordered by dependency.

---

## Intent (frozen)

Gungnr must become **easy to set up by collapsing all existential configuration into a single, one-time terminal bootstrap**, while preserving the current backend and runtime behavior.

After installation:

* The panel always starts in a fully wired state.
* GitHub OAuth login always works.
* Cloudflare tunnel, DNS, and ingress are already functional.
* The user logs in and immediately controls their own machine on the web.

Template creation via GitHub App is **explicitly post-login**, optional, and clearly scoped to that feature only.

---

## Scope constraints (do not violate)

* Backend API **must not be modified**.
* Database schema **must not be modified**.
* Existing deploy logic and job system **must not be rewritten**.
* CLI must **only** bootstrap required configuration.
* CLI must be written in **Go**, shipped as a single binary.
* Installer script exists only to install the CLI and prerequisites.
* UI changes are **cleanup and rewiring**, not feature expansion.
* Cloudflare setup is **mandatory at install time**, not deferred.
* GitHub App setup remains **inside the UI**, not in the CLI.
* No “optional paths” during bootstrap.

---

## High-level restructuring

The system is restructured into three explicit phases, with a hard boundary between them:

1. Installation (installer script)
2. Bootstrap (CLI)
3. Operation (panel UI)

Only phases 1 and 2 change behavior.
Phase 3 is UI cleanup only.

---

## Phase 1 — Installer script (repository-level)

### Purpose

Install the Gungnr CLI binary and ensure host prerequisites exist, without configuring anything.

### Implementation plan

1. Add an `install.sh` script at repository root.
2. The script must:

   * Detect OS and architecture.
   * Download the correct prebuilt `gungnr` binary (or build from source if explicitly unsupported).
   * Install it into a standard path (`/usr/local/bin/gungnr`).
3. The script must verify presence of:

   * Docker
   * Docker Compose
   * cloudflared
4. If a dependency is missing:

   * Install it using the system package manager when possible.
   * Otherwise fail with a clear message.
5. The script must **not**:

   * Write config files
   * Ask for credentials
   * Start containers
6. The script ends by printing:

   * “Run `gungnr bootstrap` to configure this machine.”

This script exists only to make `gungnr bootstrap` possible.

---

## Phase 2 — Gungnr CLI (Go binary)

### Purpose

Perform **all required one-time configuration** to make the panel runnable and fully functional.

This replaces:

* manual `.env` editing
* scattered Cloudflare setup
* SuperUser seeding confusion
* pre-login failure states

### CLI structure

The CLI exposes a single command of interest:

```
gungnr bootstrap
```

No sub-modes, no interactive menus beyond what is strictly required.

---

### Step-by-step bootstrap flow (implementation order matters)

#### Step 1 — Environment inspection

The CLI starts by inspecting the host:

* Detect home directory.
* Detect writable locations for:

* cloudflared config
* Gungnr data directory
* Detect Docker socket availability.
* Detect whether Gungnr containers are already running.

If an existing Gungnr installation is detected, abort with a clear message.

---

#### Step 2 — GitHub identity claim (SuperUser seeding)

This step replaces manual `SUPERUSER_GH_NAME` and `SUPERUSER_GH_ID` configuration.

Implementation:

1. Use GitHub OAuth **device flow** from the CLI.
2. Open browser for user authorization.
3. Poll GitHub until authorization completes.
4. Fetch authenticated user profile.
5. Extract:

* GitHub login
* Numeric GitHub user ID
6. Store these values in memory for later `.env` generation.

At this point:

* The CLI has cryptographic certainty of who is claiming the machine.
* No panel login has occurred yet.

---

#### Step 3 — Cloudflare authentication and tunnel creation

This step **must be mandatory**.

Implementation:

1. Verify `cloudflared` is installed.
2. Run:

   ```
   cloudflared tunnel login
   ```
3. Wait until Cloudflare credentials appear locally.
4. Ask for:

   * Desired tunnel name (default provided).
5. Create the tunnel via:

   ```
   cloudflared tunnel create <name>
   ```
6. Capture:

   * Tunnel UUID
   * Credentials file path
7. Generate a **complete** `config.yml`:

   * Panel ingress
   * Required catch-all rule
8. Install cloudflared as a system service.
9. Start the service and verify it is running.

---

#### Step 4 — Domain and DNS wiring

This step completes the edge.

Implementation:

1. Prompt for base domain.
2. Prompt for Cloudflare API token.
3. Validate token scopes:

   * Tunnel edit
   * DNS edit
4. Using the token:

   * Create DNS route(s) for the panel hostname.
5. Verify:

   * DNS record exists
   * Tunnel resolves

During this step: We provide all links and info about each field, just as we already do trought UI setup.

At the end of this step:

* The panel will be reachable publicly once started.


---

#### Step 5 — Filesystem materialization

Implementation:

1. Create Gungnr root directory (e.g. `~/gungnr`).
2. Create:

   * Templates directory
   * Data directory
3. These paths become the authoritative defaults.

No user choice is needed here unless the path is unwritable.

---

#### Step 6 — Environment file generation

Implementation:

1. Generate `.env` file programmatically.
2. Populate:

   * SESSION_SECRET (generated)
   * SUPERUSER_GH_NAME
   * SUPERUSER_GH_ID
   * GitHub OAuth client values
   * Cloudflare values
   * Paths and defaults

3. No placeholder values remain.

This `.env` must be sufficient for the API to start without conditional logic.

---

#### Step 7 — Start the stack

Implementation:

1. Run Docker Compose to start the panel.
2. Wait for API health endpoint to respond.
3. Verify:

   * API started successfully
   * No SuperUser boot failure
4. Print final success message:

   * Panel URL
   * Clear instruction to open browser and log in

At this point, bootstrap is complete and never needs to run again.

---

## Phase 3 — UI cleanup and rewiring (no backend changes)

### Purpose

Align the UI with the new reality: the host is already wired.

### Implementation plan

1. Remove or hide UI paths that imply:

   * initial Cloudflare setup
   * missing existential configuration
2. Assume:

   * OAuth login always works
   * Tunnel exists
   * Domain exists
3. Keep Host Settings focused on:

   * inspection
   * validation
   * minor adjustments
4. Move GitHub App configuration into a **clearly scoped section**:

   * Labelled explicitly as “Required for Create from Template”
5. If GitHub App is not configured:

   * Disable template creation UI
   * Show contextual guidance
6. Do not alter:

   * job system
   * logs
   * networking view
   * RBAC behavior

This is a client-side refactor only.

---

## Cloudflare model after changes

After bootstrap:

* cloudflared runs locally
* DNS is already wired
* Hybrid behavior is transparent
* No further Cloudflare credentials are required
* All routing operations work immediately

No Cloudflare setup remains in the critical path.

---

## Final invariant (must remain true)

After `gungnr bootstrap` completes:

* The panel can be opened on your custom domain.
* GitHub login works on first try.
* Logging in with your Github oon startup grants you Super User.
* Networking is functional.
* No feature fails due to missing credentials.
* "gungnr" run it if it's shut down.

If any of those are false, bootstrap is considered broken.

---

## Closing

This plan does **not** redesign Gungnr.
It **re-anchors** it.

Everything powerful remains.
Everything confusing moves to one place, one time, one command.

This is an implementation plan, not a vision document.
