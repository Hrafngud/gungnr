# Review Notes

## Issue: Legacy cleanup touches removed columns

On a fresh database, the new `jobs` schema no longer includes the host-worker token columns, but `CleanupLegacyHostWorker` still runs an `UPDATE` that references `host_token*` unconditionally. After `AutoMigrate` creates the slim table, this query fails with “column does not exist,” generating a warning on every startup and preventing cleanup when the legacy columns were already removed. Guarding the cleanup when columns are absent (or running it before dropping the fields) would avoid the persistent warning and ensure cleanup runs only when the legacy columns exist.

- Location: `backend/internal/db/cleanup.go` (lines 14-24)

## Issue: Nil settings dereferenced before guard

In `handleCreateTemplate` the code calls `w.settings.ResolveConfig(ctx)` before checking whether `w.settings` is nil. If the workflows service is constructed without a settings service (e.g., in tests or alternative wiring), this will panic before returning the intended "github app settings not configured" error. The nil guard should happen before any method calls on `w.settings` or the dependency should be required at construction.

- Location: `backend/internal/service/project_workflows.go` (lines 66-76)
